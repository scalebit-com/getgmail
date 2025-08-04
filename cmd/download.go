package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	gmailv1 "google.golang.org/api/gmail/v1"

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
	// Create context with overall timeout for entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
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
	processedCount := 0
	skippedCount := 0
	failedCount := 0
	
	// Special debug mode for problematic email
	debugEmailID := os.Getenv("DEBUG_EMAIL_ID")
	if debugEmailID != "" {
		log.Info(fmt.Sprintf("DEBUG MODE: Looking for email %s", debugEmailID))
		var found bool
		for _, msg := range messages {
			if msg.Id == debugEmailID {
				log.Info(fmt.Sprintf("Found target email %s, processing only this one", debugEmailID))
				messages = []*gmailv1.Message{msg}
				found = true
				break
			}
		}
		if !found {
			log.Error(fmt.Sprintf("Email %s not found in first %d messages", debugEmailID, len(messages)))
			return fmt.Errorf("target email not found")
		}
	}
	
	for i, msg := range messages {
		// Add rate limiting delay between emails (except for first one)
		if i > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		
		log.Info(fmt.Sprintf("Processing message %d/%d (ID: %s)", i+1, len(messages), msg.Id))

		// Check for context cancellation
		select {
		case <-ctx.Done():
			log.Error("Operation timeout or cancelled")
			return ctx.Err()
		default:
		}

		email, err := gmailClient.GetMessage(ctx, msg.Id)
		if err != nil {
			log.Error(fmt.Sprintf("Failed to get message %s: %v", msg.Id, err))
			failedCount++
			continue
		}

		// Check if email was already downloaded by looking for existing folder with metadata
		// Generate expected folder name without creating it first
		folderName := writer.GenerateFolderName(email)
		folderPath := filepath.Join(outputDir, folderName)
		
		// Check if metadata file exists in expected folder
		metadataPattern := filepath.Join(folderPath, "*_metadata.txt")
		matches, _ := filepath.Glob(metadataPattern)
		if len(matches) > 0 {
			log.Info(fmt.Sprintf("Email %s already downloaded, skipping", msg.Id))
			skippedCount++
			continue
		}

		// Write email to disk
		if err := writer.WriteEmail(ctx, email, outputDir); err != nil {
			log.Error(fmt.Sprintf("Failed to write message %s: %v", msg.Id, err))
			failedCount++
			continue
		}
		processedCount++
	}

	log.Info(fmt.Sprintf("Download completed. Processed: %d, Skipped: %d, Failed: %d. Emails saved to: %s", 
		processedCount, skippedCount, failedCount, outputDir))
	
	// Return error if all downloads failed
	if processedCount == 0 && skippedCount == 0 && failedCount > 0 {
		return fmt.Errorf("all email downloads failed")
	}
	
	return nil
}