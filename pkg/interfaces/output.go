package interfaces

import "context"

type OutputWriter interface {
	WriteEmail(ctx context.Context, email *EmailMessage, outputDir string) error
	ValidateOutputDir(outputDir string) error
	CreateEmailFolder(email *EmailMessage, outputDir string) (string, error)
}