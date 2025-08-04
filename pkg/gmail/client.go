package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
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

	// Create HTTP client with timeouts
	httpClient := &http.Client{
		Timeout: 60 * time.Second, // Overall request timeout
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}
	
	// Wrap the HTTP client with OAuth2
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
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

	// Add timeout for individual message fetch
	msgCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	msg, err := c.service.Users.Messages.Get(c.userID, messageID).Context(msgCtx).Do()
	if err != nil {
		// Check if it's a retryable error
		if c.isRetryableError(err) {
			// Try once more with backoff
			time.Sleep(2 * time.Second)
			msgCtx2, cancel2 := context.WithTimeout(ctx, 30*time.Second)
			defer cancel2()
			msg, err = c.service.Users.Messages.Get(c.userID, messageID).Context(msgCtx2).Do()
			if err != nil {
				return nil, fmt.Errorf("unable to retrieve message %s after retry: %v", messageID, err)
			}
		} else {
			return nil, fmt.Errorf("unable to retrieve message %s: %v", messageID, err)
		}
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
	attachmentMap := make(map[string]interfaces.Attachment)
	fmt.Printf("DEBUG: Starting attachment extraction for message %s\n", messageID)
	c.extractAttachmentsRecursive(ctx, messageID, payload, attachmentMap)
	
	// Convert map to slice
	var attachments []interfaces.Attachment
	for _, attachment := range attachmentMap {
		attachments = append(attachments, attachment)
	}
	
	fmt.Printf("DEBUG: Found %d attachments for message %s\n", len(attachments), messageID)
	return attachments
}

func (c *Client) extractAttachmentsRecursive(ctx context.Context, messageID string, payload *gmail.MessagePart, attachmentMap map[string]interfaces.Attachment) {
	// Check the payload itself for attachments
	if c.isAttachment(payload) {
		if attachment := c.processAttachment(ctx, messageID, payload); attachment != nil {
			// Use attachment ID as key to prevent duplicates
			attachmentID := payload.Body.AttachmentId
			if attachmentID != "" {
				attachmentMap[attachmentID] = *attachment
			}
		}
	}
	
	// Recursively check parts for attachments
	for _, part := range payload.Parts {
		c.extractAttachmentsRecursive(ctx, messageID, part, attachmentMap)
	}
}

func (c *Client) isAttachment(part *gmail.MessagePart) bool {
	// Skip inline images if configured
	skipInlineImages := os.Getenv("SKIP_INLINE_IMAGES") == "true"
	
	// Check for Content-ID (inline images in HTML emails)
	for _, header := range part.Headers {
		if strings.ToLower(header.Name) == "content-id" {
			// This is likely an inline image
			if part.Body != nil && part.Body.AttachmentId != "" && part.Body.Size > 0 {
				fmt.Printf("DEBUG: Found inline image with Content-ID: %s, Size: %d\n", header.Value, part.Body.Size)
				if skipInlineImages {
					fmt.Printf("DEBUG: Skipping inline image due to SKIP_INLINE_IMAGES=true\n")
					return false
				}
				return true
			}
		}
	}
	
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
	
	// HARDCODED FIX: Skip the problematic Webhallen email barcode attachment
	// This specific inline image from message 19855d64da73b5be causes the Gmail API to hang indefinitely
	// The attachment appears to be malformed with the base64 data missing from the email body
	// Issue: The attachment ID is abnormally long (400+ chars) and the API cannot retrieve it
	if messageID == "19855d64da73b5be" && strings.Contains(part.Body.AttachmentId, "ANGjdJ") {
		fmt.Printf("WARNING: Skipping known problematic attachment in Webhallen email %s\n", messageID)
		return nil
	}
	
	// Get filename from headers
	filename := c.getFilenameFromHeaders(part.Headers)
	if filename == "" {
		filename = fmt.Sprintf("attachment_%s", part.Body.AttachmentId)
	}
	
	// Truncate attachment ID for logging (they can be extremely long)
	attachIDForLog := part.Body.AttachmentId
	if len(attachIDForLog) > 50 {
		attachIDForLog = attachIDForLog[:50] + "..."
	}
	fmt.Printf("DEBUG: Processing attachment %s (ID: %s, Size: %d bytes) for message %s\n", 
		filename, attachIDForLog, part.Body.Size, messageID)
	
	// Skip very large attachments that might cause timeouts
	if part.Body.Size > 10*1024*1024 { // 10MB
		fmt.Printf("WARNING: Skipping large attachment %s (%d bytes) for message %s\n", 
			filename, part.Body.Size, messageID)
		return nil
	}
	
	// Skip attachments with suspiciously long IDs (likely corrupted)
	if len(part.Body.AttachmentId) > 500 {
		fmt.Printf("WARNING: Skipping attachment with extremely long ID (%d chars) for message %s\n", 
			len(part.Body.AttachmentId), messageID)
		return nil
	}
	
	// Download attachment data with shorter timeout for small files
	timeoutDuration := 45 * time.Second
	if part.Body.Size < 1024 { // Less than 1KB
		timeoutDuration = 10 * time.Second
	}
	attachCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()
	
	// Add small delay to avoid rate limiting
	time.Sleep(50 * time.Millisecond)
	
	fmt.Printf("DEBUG: Downloading attachment %s for message %s (timeout: %v)...\n", filename, messageID, timeoutDuration)
	attachment, err := c.service.Users.Messages.Attachments.Get(c.userID, messageID, part.Body.AttachmentId).Context(attachCtx).Do()
	if err != nil {
		// Retry once for transient errors
		if c.isRetryableError(err) {
			fmt.Printf("DEBUG: Retrying attachment download for %s...\n", filename)
			time.Sleep(2 * time.Second)
			attachCtx2, cancel2 := context.WithTimeout(ctx, 45*time.Second)
			defer cancel2()
			attachment, err = c.service.Users.Messages.Attachments.Get(c.userID, messageID, part.Body.AttachmentId).Context(attachCtx2).Do()
			if err != nil {
				fmt.Printf("Error downloading attachment %s after retry: %v\n", filename, err)
				return nil
			}
		} else {
			fmt.Printf("Error downloading attachment %s: %v\n", filename, err)
			return nil
		}
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

// isRetryableError checks if an error is retryable
func (c *Client) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for Google API errors
	if apiErr, ok := err.(*googleapi.Error); ok {
		// Retry on rate limit or server errors
		return apiErr.Code == 429 || apiErr.Code >= 500
	}
	
	// Check for timeout errors
	if strings.Contains(err.Error(), "timeout") ||
	   strings.Contains(err.Error(), "deadline exceeded") ||
	   strings.Contains(err.Error(), "connection reset") {
		return true
	}
	
	return false
}