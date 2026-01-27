# Note App

A lightweight, serverless note-taking application written in Go. Create, edit, and share note with auto-save functionality. Deploy to AWS Lambda with S3 storage or run locally with disk-based storage.

## Features

- 📝 **Simple Note Editor**: Lightweight web interface for creating and editing note
- 💾 **Auto-Save**: Automatically saves note content every second
- 🔗 **Shareable URLs**: Note is accessible via direct links with unique IDs
- 🖨️ **Print Support**: Print-friendly interface for saving note
- ⌨️ **TAB Support**: TAB key works for indentation instead of moving focus
- 🚀 **Multi-Deployment**: HTTP server, Docker container, or AWS Lambda
- 💾 **Flexible Storage**: Local disk or AWS S3 backend
- 🔒 **Secure**: Input validation and XSS protection

## Quick Start

### Local Development

**Requirements:**
- Go 1.21+

**Run:**
```bash
go run main.go
```

Open http://localhost:8080 in your browser.

**With custom settings:**
```bash
export PORT=3000
export NOTE_DIR=./my-note
go run main.go
```

### Docker

**Build:**
```bash
docker build -t note-app .
```

**Run:**
```bash
docker run -p 8080:8080 -v ./note:/note note-app
```

Then open http://localhost:8080

### AWS Lambda

**Prerequisites:**
- AWS account with IAM permissions
- S3 bucket for storing note

**Deploy with SAM (recommended):**
```bash
sam build
sam deploy --guided
```

**Manual deployment:**
1. Build binary for Lambda:
   ```bash
   GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bootstrap main.go
   zip function.zip bootstrap
   ```

2. Create Lambda function via AWS Console
3. Set environment variables:
   - `S3_BUCKET`: Your S3 bucket name
   - `S3_PREFIX`: "note" (default)
   - `AWS_REGION`: "us-east-1" (default)

4. Upload ZIP file to Lambda

## Configuration

### Environment Variables

#### Local/Docker Mode (HTTP Server)
- `PORT`: HTTP server port (default: `8080`)
- `NOTE_DIR`: Directory to store note (default: `/note`)
- `URL`: **Optional** - Public URL for sharing note (e.g., `https://note.example.com`). If not set, the domain is auto-detected from the request. Useful for reverse proxies where auto-detection may not work correctly.

#### Lambda Mode
- `S3_BUCKET`: **Required** - S3 bucket name for storing note
- `S3_PREFIX`: S3 object key prefix (default: `note`)
- `AWS_REGION`: AWS region (default: `us-east-1`)

Runtime detection is automatic:
- If `AWS_LAMBDA_FUNCTION_NAME` is set → Lambda mode with S3 storage
- Otherwise → HTTP server mode with local storage

## API

### GET /?note={noteId}

Retrieve and display a note.

**Parameters:**
- `note` (optional): Note ID (alphanumeric, 5-20 chars)

**Response:**
- HTML page with note content in a textarea
- If note doesn't exist, returns empty textarea

**Example:**
```bash
curl http://localhost:8080/?note=abc12
```

### POST /

Save or delete a note.

**Request body (JSON):**
```json
{
  "noteId": "abc12",
  "content": "Note content here"
}
```

**Response (JSON):**
```json
{
  "success": true,
  "noteId": "abc12"
}
```

**Behavior:**
- If `noteId` is empty, a random 5-character ID is generated
- If `content` is empty, the note is deleted
- Otherwise, the note is saved

**Example:**
```bash
# Create new note
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"content":"Hello World"}'

# Update note
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"noteId":"abc12","content":"Updated content"}'

# Delete note
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"noteId":"abc12","content":""}'
```

## Building

### Build for Local Execution

```bash
go build -o note-app main.go
./note-app
```

### Build with Version Info

```bash
go build -ldflags \
  "-X main.Version=v1.0.0 \
   -X main.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
   -X main.CommitHash=$(git rev-parse --short HEAD)" \
  -o note-app
```

### Build for Lambda

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build \
  -ldflags="-X main.Version=v1.0.0" \
  -o bootstrap main.go
zip function.zip bootstrap
```

### Build Docker Image

```bash
docker build -t note-app:latest .
```

## Testing

### Run All Tests

```bash
go test -v ./...
```

### Run with Coverage

```bash
go test -cover ./...
```

### Generate Coverage Report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Project Structure

```
.
├── main.go              # Entry point and runtime detection
├── handlers.go          # HTTP request handlers
├── storage_local.go     # Local file storage implementation
├── storage_s3.go        # AWS S3 storage implementation
├── lambda.go            # AWS Lambda handler and API Gateway support
├── utils.go             # Utility functions
├── *_test.go            # Unit tests
├── go.mod               # Go module definition
├── go.sum               # Dependency checksums
├── Dockerfile           # Container build definition
├── .github/
│   └── workflows/
│       ├── test.yml                 # Run tests on push/PR
│       ├── build-container.yml      # Build and push Docker image
│       └── deploy-lambda.yml        # Deploy to AWS Lambda
└── README.md            # This file
```

## CI/CD

### GitHub Actions Workflows

**test.yml:**
- Runs on every push and pull request
- Executes `go test -v -cover ./...`
- Runs `gofmt` and `go vet` checks
- Uploads coverage to Codecov

**build-container.yml:**
- Triggers on push to `main` and version tags
- Builds and pushes Docker image to GitHub Container Registry and Docker Hub
- Requires Docker Hub credentials in secrets

**deploy-lambda.yml:**
- Triggers on push to `main` and version tags
- Builds binary for Lambda (ARM64)
- Deploys to AWS Lambda
- Requires AWS credentials configured via OIDC

### Setting Up Secrets

For GitHub Actions workflows to work, configure these secrets in your repository:

**For Docker build:**
- `DOCKER_USERNAME`: Docker Hub username
- `DOCKER_PASSWORD`: Docker Hub access token

**For Lambda deployment:**
- `AWS_ROLE_TO_ASSUME`: AWS IAM role ARN for OIDC
- `AWS_REGION`: AWS region (optional, defaults to us-east-1)

## Security

- ✅ **Input Validation**: Note IDs are alphanumeric only
- ✅ **XSS Protection**: User content is HTML-escaped
- ✅ **IAM Security**: Lambda uses IAM roles, no hardcoded credentials
- ✅ **HTTPS Ready**: Works behind reverse proxies with TLS

## Performance

- **Cold Start (Lambda)**: ~500ms (Go is fast!)
- **Auto-save**: 1-second polling interval
- **Concurrent Users**: Scales automatically in Lambda mode
- **Storage**: O(1) for read/write operations

## Troubleshooting

### Note not saving locally

Ensure the `NOTE_DIR` directory exists and is writable:
```bash
mkdir -p /note
chmod 755 /note
```

### Lambda deployment fails

1. Verify S3 bucket exists and is accessible
2. Check Lambda execution role has S3 permissions
3. Ensure `S3_BUCKET` environment variable is set
4. Check CloudWatch logs for errors

### Port already in use

Change port with:
```bash
export PORT=3000
go run main.go
```

## License

MIT License - feel free to use and modify for your needs.

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## Build Info

When deployed, the app prints version information at startup:
```
Note App - Version: {Version}, BuildTime: {BuildTime}, CommitHash: {CommitHash}
```

This information is injected at build time via ldflags.
