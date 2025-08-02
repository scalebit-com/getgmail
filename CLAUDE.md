# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GetGmail is a Go CLI tool that downloads Gmail emails to local folders using the Gmail API. Each email is saved in its own directory with metadata, body content, and attachments.

## Build & Development Commands

- **Build**: `task` or `task build` - Creates binary in `target/getgmail`
- **Clean**: `task clean` - Removes contents of `target/` and `output/` directories
- **Run**: `task run` - Builds and runs with test parameters (downloads 10 emails)
- **Manual run**: `./target/getgmail download -d output -m INBOX -c 100`

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
  - `EmailMessage` - Email data structure with attachment support
  - `Attachment` - Attachment data structure with filename, MIME type, size, and binary data

- **pkg/gmail/**: Gmail API client implementation
  - Handles OAuth2 authentication and token management
  - Paginates through message lists with count limiting
  - Extracts email headers, body content from multipart messages
  - Downloads and processes email attachments automatically
  - Downloads latest emails first (Gmail API default order)

- **pkg/output/**: File system operations
  - Creates organized folder structure: `YYYY-MM-DD_HH-MM-SS_subject`
  - Sets folder modification times to match email dates (timezone-aware)
  - Sanitizes filenames and handles duplicate emails
  - Writes `metadata.txt` and `body.txt` files per email
  - Creates `attachments/` subdirectory and saves all email attachments
  - Preserves original attachment filenames with proper extensions
  - Handles duplicate attachment filenames with numbered suffixes
  - Improved date parsing with timezone suffix handling

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

The application automatically detects and downloads email attachments:
- **Detection**: Identifies attachments by checking Content-Disposition headers and attachment IDs (`pkg/gmail/client.go:230-242`)
- **Processing**: Downloads attachment data using Gmail API attachment endpoints (`pkg/gmail/client.go:245-276`)
- **Filename Extraction**: Parses filenames from Content-Disposition headers (`pkg/gmail/client.go:279-297`)
- **Storage**: Saves attachments in `attachments/` subdirectory with sanitized filenames (`pkg/output/writer.go:117-162`)
- **Metadata**: Includes attachment count and details in email metadata (`pkg/output/writer.go:107-113`)

### OAuth2 Implementation

The application implements a complete OAuth2 flow:
- Loads existing tokens from `token.json` if available
- Initiates interactive OAuth2 flow for first-time setup (`pkg/gmail/client.go:84-97`)
- Automatically saves tokens for future use (`pkg/gmail/client.go:100-108`)
- Supports token refresh through the oauth2 library
- Uses Gmail readonly scope which includes attachment access permissions