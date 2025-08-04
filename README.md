# getgmail

A command-line interface (CLI) tool written in Go that downloads Gmail emails to organized local folders. Each email is saved in its own directory with metadata, body content, and attachments. Available as both a native binary and Docker container.

## Quick Start

### Option 1: Docker (Recommended)

1. **Set up Gmail API credentials:**
   - Follow the [Gmail Go Quickstart](https://developers.google.com/gmail/api/quickstart/go)
   - Download `credentials.json` to your working directory

2. **Run with Docker:**
   ```bash
   docker run -v $(pwd):/app/data \
     -e GOOGLE_CREDENTIALS_FILE=/app/data/credentials.json \
     -e GOOGLE_TOKEN_FILE=/app/data/token.json \
     perarneng/getgmail:latest download -d /app/data/output -m INBOX -c 50
   ```

### Option 2: Native Binary

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

### Native Development
- `task` - Build the project
- `task clean` - Clean build artifacts and output
- `task run` - Build and run with test parameters (10 emails)

### Docker Commands
- `task docker-build` - Build Docker image with version tags
- `task docker-push` - Push Docker image to registry
- `task docker-run` - Run Docker container with mounted directory

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
- **Optimized Performance**: Fast duplicate detection without redundant API calls or folder creation
- **Batch Processing**: Efficiently handles 100+ emails with incremental download support
- **Organized Storage**: Creates folders named `YYYY-MM-DD_HH-MM-SS_subject` 
- **Timezone Aware**: Folder modification times match email dates in your local timezone
- **Smart Deduplication**: Pre-checks for existing emails before folder creation or API calls
- **Robust Date Parsing**: Handles various email date formats and timezone suffixes
- **Clean Output**: Sanitizes filenames and handles long subjects
- **Attachment Support**: Automatically downloads and saves email attachments with deduplication
- **Consistent File Naming**: All files use prefixed naming with date-time-subject format
- **Docker Support**: Multi-stage optimized Docker image (51.4MB) with security hardening
- **Network Resilience**: Handles temporary network issues - simply re-run to continue
- **Timeout Protection**: Configurable timeouts prevent hanging on problematic emails
- **Rate Limiting**: Built-in delays prevent API throttling
- **Retry Logic**: Automatic retry with exponential backoff for transient failures

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
- Smart deduplication prevents downloading the same attachment multiple times
- Attachment details included in `metadata.txt`

## Docker Usage

### Available Images
- `perarneng/getgmail:latest` - Latest stable version
- `perarneng/getgmail:1.4.0` - Specific version tag

### Basic Usage
```bash
# Interactive mode (for first-time OAuth setup)
docker run -it -v $(pwd):/app/data \
  -e GOOGLE_CREDENTIALS_FILE=/app/data/credentials.json \
  -e GOOGLE_TOKEN_FILE=/app/data/token.json \
  perarneng/getgmail:latest download -d /app/data/output -m INBOX -c 10

# Production usage
docker run -v $(pwd):/app/data \
  -e GOOGLE_CREDENTIALS_FILE=/app/data/credentials.json \
  -e GOOGLE_TOKEN_FILE=/app/data/token.json \
  perarneng/getgmail:latest download -d /app/data/output -m INBOX -c 100
```

### Environment Variables
- `GOOGLE_CREDENTIALS_FILE` - Path to OAuth2 credentials JSON file
- `GOOGLE_TOKEN_FILE` - Path to token file (defaults to "token.json")

### Volume Mounting
Mount your working directory to `/app/data` to:
- Provide credential files to the container
- Persist downloaded emails to your local filesystem
- Maintain OAuth tokens between runs

## Requirements

### Docker (Recommended)
- Docker engine
- Gmail API credentials

### Native Development
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

## Performance Notes

- **Typical Performance**: Downloads 50-100 new emails in 30-60 seconds
- **Incremental Downloads**: Subsequent runs skip already-downloaded emails in seconds
- **Gmail API Quotas**: Uses ~5-10 quota units per email (well below the 15,000/minute limit)
- **Large Batches**: If downloading stops unexpectedly, simply re-run - the tool will continue from where it left off
- **Optimizations**: The tool checks for existing emails before creating folders or making API calls
- **Timeout Handling**: Individual operations have timeouts (30s for emails, 45s for attachments)
- **Rate Limiting**: 100ms delay between emails, 50ms between attachments to avoid API throttling

## Known Issues

- Some malformed inline images in emails may be skipped to prevent API hanging
- Very large attachments (>10MB) are skipped by default to prevent timeouts

## Version History

- **v1.4.0** - Improved timeout handling, rate limiting, and fix for problematic inline images
- **v1.3.0** - Enhanced Docker support and attachment improvements  
- **v1.2.0** - Optimized duplicate detection and performance
- **v1.1.0** - Added attachment support
- **v1.0.0** - Initial release