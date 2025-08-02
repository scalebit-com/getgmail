# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GetGmail is a Go CLI tool that downloads Gmail emails to local folders using the Gmail API. Each email is saved in its own directory with metadata, body content, and attachments.

## Build & Development Commands

- **Build**: `task` or `task build` - Creates binary in `target/getgmail`
- **Clean**: `task clean` - Removes contents of `target/` and `output/` directories
- **Run**: `task run` - Builds and runs with test parameters (downloads 10 emails)
- **Manual run**: `./target/getgmail download -d output -m INBOX -c 100`

### Docker Commands
- **Docker Build**: `task docker-build` - Builds Docker image with both latest and version tags
- **Docker Push**: `task docker-push` - Pushes image to Docker Hub registry 
- **Docker Run**: `task docker-run` - Runs container with mounted directory and environment variables
- **Manual Docker**: `docker run -v $(pwd):/app/data -e GOOGLE_CREDENTIALS_FILE=/app/data/credentials.json -e GOOGLE_TOKEN_FILE=/app/data/token.json perarneng/getgmail:latest download -d /app/data/output -m INBOX -c 100`

## Environment Setup

### Gmail API Setup
1. Follow the [Gmail Go Quickstart](https://developers.google.com/gmail/api/quickstart/go) to:
   - Enable the Gmail API in Google Cloud Console
   - Create OAuth2 credentials
   - Download the `credentials.json` file to the project root

2. Set environment variables:
   - `GOOGLE_CREDENTIALS_FILE=credentials.json` - Path to OAuth2 credentials JSON file
   - `GOOGLE_TOKEN_FILE=token.json` - Path to token file (optional, defaults to "token.json")
   - Optional `.env` file support via godotenv

3. On first run, the application will:
   - Prompt you to visit an OAuth2 authorization URL
   - Ask you to enter the authorization code
   - Save the token to `token.json` for future use

## Architecture

### Core Components

- **cmd/**: Cobra CLI commands
  - `root.go` - Main CLI setup
  - `download.go` - Email download command with flags:
    - `-m/--mailbox` - Gmail mailbox/label (default: INBOX)
    - `-d/--output-dir` - Output directory (required)
    - `-c/--count` - Maximum number of emails to download (default: 100)

- **pkg/interfaces/**: Interface definitions for loose coupling
  - `GmailClient` - Gmail API operations
  - `Logger` - Structured logging interface  
  - `OutputWriter` - File system operations
  - `EmailMessage` - Email data structure with attachment support and body MIME type detection
  - `Attachment` - Attachment data structure with filename, MIME type, size, and binary data

- **pkg/gmail/**: Gmail API client implementation
  - Handles OAuth2 authentication and token management
  - Paginates through message lists with count limiting
  - Extracts email headers, body content from multipart messages with MIME type detection
  - Downloads and processes email attachments automatically
  - Downloads latest emails first (Gmail API default order)

- **pkg/output/**: File system operations
  - Creates organized folder structure: `YYYY-MM-DD_HH-MM-SS_subject`
  - Sets folder modification times to match email dates (timezone-aware)
  - Sanitizes filenames and handles duplicate emails
  - Uses consistent prefixed naming for all files: `{prefix}_filename`
  - Writes `{prefix}_metadata.txt` and `{prefix}_body.html` (always HTML format)
  - Saves attachments directly in email directory as `{prefix}_{attachment_filename}`
  - Preserves original attachment filenames with proper extensions after prefix
  - Handles duplicate attachment filenames with numbered suffixes
  - Improved date parsing with timezone suffix handling
  - Subject length truncation to prevent filesystem filename length issues

- **pkg/logger/**: Colored console logging with timestamps

### Key Dependencies

- `github.com/spf13/cobra` - CLI framework
- `google.golang.org/api/gmail/v1` - Gmail API client
- `golang.org/x/oauth2` - OAuth2 authentication
- `github.com/fatih/color` - Colored terminal output
- `github.com/joho/godotenv` - Environment variable loading

### Data Flow

1. CLI parses flags (including count limit) and loads environment variables
2. Gmail client authenticates via OAuth2 and connects to API
3. Lists messages from specified mailbox with efficient pagination and count limiting
4. For each message: fetches full content, creates folder, writes metadata/body/attachments
5. Downloads attachments using Gmail API attachment endpoints with base64 decoding
6. Sets folder modification time to email date after writing all files
7. Skips already downloaded emails based on existing metadata files

### Attachment Implementation

The application automatically detects and downloads email attachments with deduplication:
- **Detection**: Identifies attachments by checking Content-Disposition headers and attachment IDs (`pkg/gmail/client.go:286-298`)
- **Deduplication**: Uses attachment ID mapping to prevent duplicate downloads (`pkg/gmail/client.go:268-297`) 
- **Processing**: Downloads attachment data using Gmail API attachment endpoints (`pkg/gmail/client.go:301-330`)
- **Filename Extraction**: Parses filenames from Content-Disposition headers (`pkg/gmail/client.go:333-351`)
- **Storage**: Saves attachments directly in email directory with prefixed filenames (`pkg/output/writer.go`)
- **Naming**: Uses consistent prefix format: `{date-time-subject}_{attachment_filename}`
- **Metadata**: Includes attachment count and details in email metadata

### MIME Type Detection and Body Handling

The application automatically detects email body MIME types and uses consistent naming:
- **MIME Type Extraction**: Extracts MIME type from Gmail API message parts (`pkg/gmail/client.go:189-210`)
- **Consistent File Naming**: All body files use `{prefix}_body.html` format for uniform handling
- **MIME Type Preservation**: Original body MIME type is preserved in `metadata.txt` for reference
- **HTML Processing**: Plain text emails are wrapped in HTML for consistent processing
- **Structure Update**: `EmailMessage` struct includes `BodyMimeType` field (`pkg/interfaces/gmail.go:23`)

### OAuth2 Implementation

The application implements a complete OAuth2 flow:
- Loads existing tokens from `token.json` if available
- Initiates interactive OAuth2 flow for first-time setup (`pkg/gmail/client.go:84-97`)
- Automatically saves tokens for future use (`pkg/gmail/client.go:100-108`)
- Supports token refresh through the oauth2 library
- Uses Gmail readonly scope which includes attachment access permissions

### Docker Implementation

The project includes Docker containerization with multi-stage build optimization:
- **Dockerfile**: Multi-stage build using Go 1.24-alpine base with final Alpine runtime (`Dockerfile`)
- **Image Size**: Optimized to 51.4MB using minimal Alpine Linux base
- **Security**: Runs as non-root user (appuser:1000) with minimal privileges
- **Environment**: Supports environment variables for credential file paths
- **Volume Mounting**: Allows mounting host directories for credential and output file access
- **Registry**: Published to Docker Hub as `perarneng/getgmail:latest` and `perarneng/getgmail:1.0.0`
- **Exclusions**: `.dockerignore` prevents secrets (credentials.json, token.json, .env) from being included in image
- **Tasks**: Automated build and push via Taskfile.yml with version tagging from `version.txt`