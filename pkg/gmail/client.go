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
		ID:      msg.Id,
		Headers: make(map[string]string),
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
	email.Body = c.extractBody(msg.Payload)

	return email, nil
}

func (c *Client) extractBody(payload *gmail.MessagePart) string {
	if payload.Body != nil && payload.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(payload.Body.Data)
		if err == nil {
			return string(data)
		}
	}

	// Check parts for multipart messages
	for _, part := range payload.Parts {
		if part.MimeType == "text/plain" || part.MimeType == "text/html" {
			if part.Body != nil && part.Body.Data != "" {
				data, err := base64.URLEncoding.DecodeString(part.Body.Data)
				if err == nil {
					return string(data)
				}
			}
		}
	}

	return ""
}