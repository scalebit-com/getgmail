# getgmail

A command-line interface (CLI) tool written in Go that downloads Gmail emails to organized local folders. Each email is saved in its own directory with metadata and body content, and folder timestamps match the email dates.

## Quick Start

1. **Build the project:**
   ```bash
   task
   ```

2. **Set up Gmail API credentials:**
   - Follow the [Gmail Go Quickstart](https://developers.google.com/gmail/api/quickstart/go)
   - Download `credentials.json` to the project root
   - Set `GOOGLE_CREDENTIALS_FILE=credentials.json` in `.env` file

3. **Download emails:**
   ```bash
   # Download 10 emails (test run)
   task run
   
   # Download 100 emails (default)
   ./target/getgmail download -d output -m INBOX
   
   # Download specific number of emails
   ./target/getgmail download -d output -m INBOX -c 50
   ```

## Commands

- `task` - Build the project
- `task clean` - Clean build artifacts and output
- `task run` - Build and run with test parameters (10 emails)

## Command Line Options

```bash
./target/getgmail download [flags]
```

### Flags
- `-d, --output-dir` - Output directory for downloaded emails (required)
- `-m, --mailbox` - Gmail mailbox/label to download from (default: "INBOX")
- `-c, --count` - Maximum number of emails to download (default: 100)

## Features

- **OAuth2 Authentication**: Secure Gmail API access with automatic token management
- **Efficient Download**: Downloads latest emails first with configurable count limits
- **Organized Storage**: Creates folders named `YYYY-MM-DD_HH-MM-SS_subject` 
- **Timezone Aware**: Folder modification times match email dates in your local timezone
- **Smart Deduplication**: Skips already downloaded emails
- **Robust Date Parsing**: Handles various email date formats and timezone suffixes
- **Clean Output**: Sanitizes filenames and handles long subjects
- **Attachment Support**: Automatically downloads and saves email attachments
- **Consistent File Naming**: All files use prefixed naming with date-time-subject format

## Output Structure

Each email is saved in its own folder with consistent prefixed naming:
```
output/
├── 2025-08-01_04-39-03_Receipt-for-Your-Payment/
│   ├── 2025-08-01_04-39-03_Receipt-for-Your-Payment_metadata.txt
│   ├── 2025-08-01_04-39-03_Receipt-for-Your-Payment_body.html
│   ├── 2025-08-01_04-39-03_Receipt-for-Your-Payment_invoice.pdf
│   └── 2025-08-01_04-39-03_Receipt-for-Your-Payment_receipt.jpg
└── 2025-08-01_05-19-14_Important-Document/
    ├── 2025-08-01_05-19-14_Important-Document_metadata.txt
    ├── 2025-08-01_05-19-14_Important-Document_body.html
    └── 2025-08-01_05-19-14_Important-Document_document.docx
```

### File Naming Convention

All files within an email directory use a consistent prefix format:
- **Prefix Format**: `YYYY-MM-DD_HH-MM-SS_subject_`
- **Metadata**: `{prefix}_metadata.txt`
- **Body**: `{prefix}_body.html` (always HTML format)
- **Attachments**: `{prefix}_{original_filename}`

### Email Body Content

- **Consistent Extensions**: All email body files are saved as `.html` for uniform handling
- **MIME Type Metadata**: The original body MIME type is preserved in `metadata.txt`
- **HTML Wrapping**: Plain text emails are wrapped in HTML for consistent processing

### Attachment Handling

- Attachments are automatically detected and downloaded
- Saved directly in the email directory (no subdirectory)
- Prefixed with the same date-time-subject format for easy identification
- Original filenames and extensions are preserved after the prefix
- Duplicate filenames are handled with numbered suffixes
- Attachment details included in `metadata.txt`

## Requirements

- Go 1.24.5 or later
- Task runner (go-task)
- Gmail API credentials

## Environment Setup

Create a `.env` file in the project root:
```
GOOGLE_CREDENTIALS_FILE=credentials.json
GOOGLE_TOKEN_FILE=token.json
```

On first run, the application will guide you through the OAuth2 authorization process.