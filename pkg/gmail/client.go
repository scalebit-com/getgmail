package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/perarneng/getgmail/pkg/interfaces"
)

type Client struct {
	service *gmail.Service
	userID  string
}

func NewClient() interfaces.GmailClient {
	return &Client{
		userID: "me",
	}
}

func (c *Client) Connect(ctx context.Context) error {
	credentialsFile := os.Getenv("GOOGLE_CREDENTIALS_FILE")
	if credentialsFile == "" {
		return fmt.Errorf("GOOGLE_CREDENTIALS_FILE environment variable not set")
	}

	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		return fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		return fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	tokenFile := os.Getenv("GOOGLE_TOKEN_FILE")
	if tokenFile == "" {
		tokenFile = "token.json"
	}

	tok, err := c.tokenFromFile(tokenFile)
	if err != nil {
		// If token doesn't exist, start OAuth2 flow
		tok, err = c.getTokenFromWeb(config)
		if err != nil {
			return fmt.Errorf("unable to get token from web: %v", err)
		}
		c.saveToken(tokenFile, tok)
	}

	client := config.Client(ctx, tok)
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("unable to retrieve Gmail client: %v", err)
	}

	c.service = srv
	return nil
}

func (c *Client) tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func (c *Client) getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %v", err)
	}
	return tok, nil
}

func (c *Client) saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func (c *Client) ListMessages(ctx context.Context, mailbox string, maxResults int64) ([]*gmail.Message, error) {
	if c.service == nil {
		return nil, fmt.Errorf("gmail service not connected")
	}

	var messages []*gmail.Message
	pageToken := ""
	remaining := maxResults

	for remaining > 0 {
		call := c.service.Users.Messages.List(c.userID).LabelIds(mailbox)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		
		// Set page size to remaining count or max page size (500)
		pageSize := remaining
		if pageSize > 500 {
			pageSize = 500
		}
		call = call.MaxResults(pageSize)

		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve messages: %v", err)
		}

		messages = append(messages, resp.Messages...)
		remaining -= int64(len(resp.Messages))

		if resp.NextPageToken == "" || remaining <= 0 {
			break
		}
		pageToken = resp.NextPageToken
	}

	return messages, nil
}

func (c *Client) GetMessage(ctx context.Context, messageID string) (*interfaces.EmailMessage, error) {
	if c.service == nil {
		return nil, fmt.Errorf("gmail service not connected")
	}

	msg, err := c.service.Users.Messages.Get(c.userID, messageID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve message %s: %v", messageID, err)
	}

	email := &interfaces.EmailMessage{
		ID:          msg.Id,
		Headers:     make(map[string]string),
		Attachments: []interfaces.Attachment{},
	}

	// Extract headers
	for _, header := range msg.Payload.Headers {
		email.Headers[header.Name] = header.Value
		switch strings.ToLower(header.Name) {
		case "subject":
			email.Subject = header.Value
		case "from":
			email.From = header.Value
		case "to":
			email.To = header.Value
		case "date":
			email.Date = header.Value
		}
	}

	// Extract body
	email.Body, email.BodyMimeType = c.extractBody(msg.Payload)

	// Extract attachments
	email.Attachments = c.extractAttachments(ctx, msg.Id, msg.Payload)

	return email, nil
}

func (c *Client) extractBody(payload *gmail.MessagePart) (string, string) {
	var htmlContent, plainContent string
	var htmlMime, plainMime string

	c.recursiveExtractBody(payload, &htmlContent, &plainContent, &htmlMime, &plainMime)

	// Prioritize HTML content if available
	if htmlContent != "" {
		return htmlContent, htmlMime
	}

	// If only plain text, wrap it in HTML structure
	if plainContent != "" {
		wrappedHTML := c.wrapPlainTextAsHTML(plainContent)
		return wrappedHTML, "text/html"
	}

	return "", "text/html"
}

func (c *Client) recursiveExtractBody(payload *gmail.MessagePart, htmlContent, plainContent, htmlMime, plainMime *string) {
	// Check if current payload has body data
	if payload.Body != nil && payload.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(payload.Body.Data)
		if err == nil {
			content := string(data)
			if payload.MimeType == "text/html" && *htmlContent == "" {
				*htmlContent = content
				*htmlMime = payload.MimeType
			} else if payload.MimeType == "text/plain" && *plainContent == "" {
				*plainContent = content
				*plainMime = payload.MimeType
			}
		}
	}

	// Recursively check all parts
	for _, part := range payload.Parts {
		c.recursiveExtractBody(part, htmlContent, plainContent, htmlMime, plainMime)
	}
}

func (c *Client) wrapPlainTextAsHTML(plainText string) string {
	// Escape HTML characters in plain text
	escaped := strings.ReplaceAll(plainText, "&", "&amp;")
	escaped = strings.ReplaceAll(escaped, "<", "&lt;")
	escaped = strings.ReplaceAll(escaped, ">", "&gt;")
	escaped = strings.ReplaceAll(escaped, "\"", "&quot;")
	escaped = strings.ReplaceAll(escaped, "'", "&#39;")

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>Email Content</title>
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
			line-height: 1.6;
			max-width: 800px;
			margin: 20px;
			padding: 20px;
		}
		pre {
			white-space: pre-wrap;
			word-wrap: break-word;
			background-color: #f5f5f5;
			padding: 15px;
			border-radius: 5px;
			border: 1px solid #ddd;
		}
	</style>
</head>
<body>
	<pre>%s</pre>
</body>
</html>`, escaped)
}

func (c *Client) extractAttachments(ctx context.Context, messageID string, payload *gmail.MessagePart) []interfaces.Attachment {
	var attachments []interfaces.Attachment
	
	// Check the payload itself for attachments
	if c.isAttachment(payload) {
		if attachment := c.processAttachment(ctx, messageID, payload); attachment != nil {
			attachments = append(attachments, *attachment)
		}
	}
	
	// Recursively check parts for attachments
	for _, part := range payload.Parts {
		attachments = append(attachments, c.extractAttachments(ctx, messageID, part)...)
	}
	
	return attachments
}

func (c *Client) isAttachment(part *gmail.MessagePart) bool {
	// Check if this part has a filename in headers
	for _, header := range part.Headers {
		if strings.ToLower(header.Name) == "content-disposition" {
			if strings.Contains(strings.ToLower(header.Value), "attachment") ||
				strings.Contains(strings.ToLower(header.Value), "filename") {
				return true
			}
		}
	}
	
	// Check if it has an attachment ID and body size > 0
	return part.Body != nil && part.Body.AttachmentId != "" && part.Body.Size > 0
}

func (c *Client) processAttachment(ctx context.Context, messageID string, part *gmail.MessagePart) *interfaces.Attachment {
	if part.Body == nil || part.Body.AttachmentId == "" {
		return nil
	}
	
	// Get filename from headers
	filename := c.getFilenameFromHeaders(part.Headers)
	if filename == "" {
		filename = fmt.Sprintf("attachment_%s", part.Body.AttachmentId)
	}
	
	// Download attachment data
	attachment, err := c.service.Users.Messages.Attachments.Get(c.userID, messageID, part.Body.AttachmentId).Context(ctx).Do()
	if err != nil {
		fmt.Printf("Error downloading attachment %s: %v\n", filename, err)
		return nil
	}
	
	// Decode attachment data
	data, err := base64.URLEncoding.DecodeString(attachment.Data)
	if err != nil {
		fmt.Printf("Error decoding attachment %s: %v\n", filename, err)
		return nil
	}
	
	return &interfaces.Attachment{
		Filename:     filename,
		MimeType:     part.MimeType,
		Size:         part.Body.Size,
		Data:         data,
		AttachmentID: part.Body.AttachmentId,
	}
}

func (c *Client) getFilenameFromHeaders(headers []*gmail.MessagePartHeader) string {
	for _, header := range headers {
		if strings.ToLower(header.Name) == "content-disposition" {
			// Parse filename from Content-Disposition header
			value := header.Value
			if idx := strings.Index(strings.ToLower(value), "filename="); idx != -1 {
				filename := value[idx+9:]
				// Remove quotes if present
				filename = strings.Trim(filename, `"`)
				// Remove any trailing parameters
				if idx := strings.Index(filename, ";"); idx != -1 {
					filename = filename[:idx]
				}
				return strings.TrimSpace(filename)
			}
		}
	}
	return ""
}