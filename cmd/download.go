package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/perarneng/getgmail/pkg/gmail"
	"github.com/perarneng/getgmail/pkg/logger"
	"github.com/perarneng/getgmail/pkg/output"
)

var (
	mailbox   string
	outputDir string
	count     int
)

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download emails from Gmail to local folder",
	Long:  `Download emails from a specified Gmail mailbox to a local directory. Each email is saved in its own folder with metadata and body.`,
	RunE:  runDownload,
}

func init() {
	downloadCmd.Flags().StringVarP(&mailbox, "mailbox", "m", "INBOX", "Gmail mailbox/label to download from")
	downloadCmd.Flags().StringVarP(&outputDir, "output-dir", "d", "", "Output directory for downloaded emails (required)")
	downloadCmd.Flags().IntVarP(&count, "count", "c", 100, "Maximum number of emails to download")
	downloadCmd.MarkFlagRequired("output-dir")
	
	rootCmd.AddCommand(downloadCmd)
}

func runDownload(cmd *cobra.Command, args []string) error {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// Don't fail if .env doesn't exist, just continue
	}

	// Initialize logger
	log := logger.NewLogger()

	// Validate output directory
	writer := output.NewFileWriter(log)
	if err := writer.ValidateOutputDir(outputDir); err != nil {
		return err
	}

	// Initialize Gmail client
	gmailClient := gmail.NewClient()
	
	log.Info("Connecting to Gmail API...")
	ctx := context.Background()
	if err := gmailClient.Connect(ctx); err != nil {
		log.Error(fmt.Sprintf("Failed to connect to Gmail: %v", err))
		return err
	}

	log.Info(fmt.Sprintf("Connected successfully, downloading from mailbox: %s (max %d emails)", mailbox, count))

	// List messages
	log.Info(fmt.Sprintf("Fetching message list (max %d messages)...", count))
	messages, err := gmailClient.ListMessages(ctx, mailbox, int64(count))
	if err != nil {
		log.Error(fmt.Sprintf("Failed to list messages: %v", err))
		return err
	}

	log.Info(fmt.Sprintf("Found %d messages to process", len(messages)))

	// Download each message
	for i, msg := range messages {
		log.Info(fmt.Sprintf("Processing message %d/%d (ID: %s)", i+1, len(messages), msg.Id))

		email, err := gmailClient.GetMessage(ctx, msg.Id)
		if err != nil {
			log.Error(fmt.Sprintf("Failed to get message %s: %v", msg.Id, err))
			continue
		}

		// Check if email already exists
		folderPath, err := writer.CreateEmailFolder(email, outputDir)
		if err != nil {
			log.Error(fmt.Sprintf("Failed to create folder for message %s: %v", msg.Id, err))
			continue
		}

		// Check if email was already downloaded
		metadataFile := fmt.Sprintf("%s/metadata.txt", folderPath)
		if _, err := os.Stat(metadataFile); err == nil {
			log.Info(fmt.Sprintf("Email %s already downloaded, skipping", msg.Id))
			continue
		}

		// Write email to disk
		if err := writer.WriteEmail(ctx, email, outputDir); err != nil {
			log.Error(fmt.Sprintf("Failed to write message %s: %v", msg.Id, err))
			continue
		}
	}

	log.Info(fmt.Sprintf("Download completed. Emails saved to: %s", outputDir))
	return nil
}