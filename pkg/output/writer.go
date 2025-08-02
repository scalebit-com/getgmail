package output

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/perarneng/getgmail/pkg/interfaces"
)

type FileWriter struct {
	logger interfaces.Logger
}

func NewFileWriter(logger interfaces.Logger) interfaces.OutputWriter {
	return &FileWriter{
		logger: logger,
	}
}

func (w *FileWriter) ValidateOutputDir(outputDir string) error {
	info, err := os.Stat(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			w.logger.Error(fmt.Sprintf("Output directory does not exist: %s", outputDir))
			return fmt.Errorf("output directory does not exist: %s", outputDir)
		}
		return fmt.Errorf("error checking output directory: %v", err)
	}

	if !info.IsDir() {
		w.logger.Error(fmt.Sprintf("Output path is not a directory: %s", outputDir))
		return fmt.Errorf("output path is not a directory: %s", outputDir)
	}

	w.logger.Info(fmt.Sprintf("Output directory validated: %s", outputDir))
	return nil
}

// generateFilePrefix creates a consistent prefix for all files in an email directory
// Format: YYYY-MM-DD_HH-MM-SS_subject
// Ensures total length doesn't exceed 255 characters for filesystem compatibility
func (w *FileWriter) generateFilePrefix(email *interfaces.EmailMessage) string {
	// Parse date
	date := w.parseEmailDate(email.Date)
	dateStr := date.Format("2006-01-02_15-04-05")

	// Clean subject for filesystem
	subject := w.sanitizeForFilename(email.Subject)
	if subject == "" {
		subject = "no-subject"
	}

	// Calculate max subject length to keep total under 255 chars
	// Account for: dateStr (19) + "_" (1) + longest suffix ("_metadata.txt" = 13) = 33
	// Leave some buffer for attachment filenames
	maxSubjectLen := 200 - len(dateStr) - 1 // 200 to leave room for suffixes
	if len(subject) > maxSubjectLen {
		subject = subject[:maxSubjectLen]
	}

	return fmt.Sprintf("%s_%s", dateStr, subject)
}

func (w *FileWriter) CreateEmailFolder(email *interfaces.EmailMessage, outputDir string) (string, error) {
	folderName := w.generateFilePrefix(email)
	folderPath := filepath.Join(outputDir, folderName)

	// Check if folder already exists
	folderExists := false
	if _, err := os.Stat(folderPath); err == nil {
		w.logger.Info(fmt.Sprintf("Email folder already exists: %s", folderName))
		folderExists = true
	}

	// Create folder if it doesn't exist
	if !folderExists {
		err := os.MkdirAll(folderPath, 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create email folder: %v", err)
		}
		w.logger.Info(fmt.Sprintf("Created email folder: %s", folderName))
	}

	return folderPath, nil
}

func (w *FileWriter) WriteEmail(ctx context.Context, email *interfaces.EmailMessage, outputDir string) error {
	folderPath, err := w.CreateEmailFolder(email, outputDir)
	if err != nil {
		return err
	}

	// Generate consistent file prefix
	filePrefix := w.generateFilePrefix(email)

	// Write email metadata
	metadataPath := filepath.Join(folderPath, filePrefix+"_metadata.txt")
	metadataContent := fmt.Sprintf(`Email ID: %s
Subject: %s
From: %s
To: %s
Date: %s
Body MIME Type: %s
Attachments: %d

Headers:
`, email.ID, email.Subject, email.From, email.To, email.Date, email.BodyMimeType, len(email.Attachments))

	for key, value := range email.Headers {
		metadataContent += fmt.Sprintf("%s: %s\n", key, value)
	}

	// Add attachment details to metadata
	if len(email.Attachments) > 0 {
		metadataContent += "\nAttachments:\n"
		for i, attachment := range email.Attachments {
			metadataContent += fmt.Sprintf("  %d. %s (%s, %d bytes)\n", 
				i+1, attachment.Filename, attachment.MimeType, attachment.Size)
		}
	}

	err = os.WriteFile(metadataPath, []byte(metadataContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write metadata: %v", err)
	}

	// Write email body - always save as HTML since we now wrap plain text in HTML
	bodyPath := filepath.Join(folderPath, filePrefix+"_body.html")
	err = os.WriteFile(bodyPath, []byte(email.Body), 0644)
	if err != nil {
		return fmt.Errorf("failed to write email body: %v", err)
	}

	// Write attachments directly in email directory with prefix
	if len(email.Attachments) > 0 {
		for i, attachment := range email.Attachments {
			filename := attachment.Filename
			if filename == "" {
				filename = fmt.Sprintf("attachment_%d", i+1)
			}
			
			// Sanitize filename
			filename = w.sanitizeForFilename(filename)
			if filename == "" {
				filename = fmt.Sprintf("attachment_%d", i+1)
			}

			// Create attachment path with prefix
			attachmentFilename := fmt.Sprintf("%s_%s", filePrefix, filename)
			attachmentPath := filepath.Join(folderPath, attachmentFilename)
			
			// Handle duplicate filenames by adding a counter
			originalPath := attachmentPath
			counter := 1
			for {
				if _, err := os.Stat(attachmentPath); os.IsNotExist(err) {
					break
				}
				ext := filepath.Ext(originalPath)
				base := strings.TrimSuffix(originalPath, ext)
				attachmentPath = fmt.Sprintf("%s_%d%s", base, counter, ext)
				counter++
			}

			err = os.WriteFile(attachmentPath, attachment.Data, 0644)
			if err != nil {
				w.logger.Warn(fmt.Sprintf("Failed to write attachment %s: %v", attachmentFilename, err))
				continue
			}
			
			w.logger.Info(fmt.Sprintf("Wrote attachment: %s (%d bytes)", attachmentFilename, len(attachment.Data)))
		}
		
		w.logger.Info(fmt.Sprintf("Wrote %d attachments to %s", len(email.Attachments), folderPath))
	}

	// Set folder modification time to email date AFTER writing all files
	date := w.parseEmailDate(email.Date)
	w.logger.Debug(fmt.Sprintf("Setting folder timestamp to: %s", date.Format(time.RFC3339)))
	err = os.Chtimes(folderPath, date, date)
	if err != nil {
		w.logger.Warn(fmt.Sprintf("Failed to set folder timestamp: %v", err))
	} else {
		w.logger.Debug(fmt.Sprintf("Successfully set folder timestamp"))
	}

	w.logger.Info(fmt.Sprintf("Wrote email %s to %s", email.ID, folderPath))
	return nil
}

func (w *FileWriter) parseEmailDate(dateStr string) time.Time {
	// Clean up date string - remove timezone suffixes like (UTC), (GMT), etc.
	cleanDateStr := regexp.MustCompile(`\s*\([^)]+\)\s*$`).ReplaceAllString(dateStr, "")
	
	// Try common email date formats
	formats := []string{
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		time.RFC1123Z,  // "Mon, 02 Jan 2006 15:04:05 -0700"
		time.RFC1123,   // "Mon, 02 Jan 2006 15:04:05 MST"
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, cleanDateStr); err == nil {
			return t
		}
	}

	// If all parsing fails, return current time
	w.logger.Warn(fmt.Sprintf("Could not parse date '%s', using current time", dateStr))
	return time.Now()
}

func (w *FileWriter) sanitizeForFilename(s string) string {
	// Remove or replace invalid characters for filenames, but keep dots for extensions
	reg := regexp.MustCompile(`[^\w\s.-]`)
	cleaned := reg.ReplaceAllString(s, "")
	
	// Replace spaces and multiple dashes with single dash
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, "-")
	cleaned = regexp.MustCompile(`-+`).ReplaceAllString(cleaned, "-")
	
	// Trim dashes from start and end
	cleaned = strings.Trim(cleaned, "-")
	
	return cleaned
}