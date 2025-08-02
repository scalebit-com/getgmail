package interfaces

import (
	"context"
	"google.golang.org/api/gmail/v1"
)

type EmailMessage struct {
	ID      string
	Subject string
	Date    string
	From    string
	To      string
	Body    string
	Headers map[string]string
}

type GmailClient interface {
	ListMessages(ctx context.Context, mailbox string) ([]*gmail.Message, error)
	GetMessage(ctx context.Context, messageID string) (*EmailMessage, error)
	Connect(ctx context.Context) error
}