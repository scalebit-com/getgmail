package interfaces

import (
	"context"
	"google.golang.org/api/gmail/v1"
)

type Attachment struct {
	Filename    string
	MimeType    string
	Size        int64
	Data        []byte
	AttachmentID string
}

type EmailMessage struct {
	ID           string
	Subject      string
	Date         string
	From         string
	To           string
	Body         string
	BodyMimeType string
	Headers      map[string]string
	Attachments  []Attachment
}

type GmailClient interface {
	ListMessages(ctx context.Context, mailbox string, maxResults int64) ([]*gmail.Message, error)
	GetMessage(ctx context.Context, messageID string) (*EmailMessage, error)
	Connect(ctx context.Context) error
}