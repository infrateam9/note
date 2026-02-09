# Agent Guidelines for Note App

## Project Overview

A lightweight, serverless note-taking web app written in Go. Single binary, no JS frameworks, no build step. The entire HTML/CSS/JS UI is an inline template string in `handlers.go` (`renderHTML` function).

## Architecture

- **Language**: Go 1.24+
- **Frontend**: Inline HTML/CSS/JS template in `renderHTML()` within `handlers.go` — no separate static files (except `favicon.ico`)
- **Storage**: Pluggable `Storage` interface — `LocalStorage` (disk) or `S3Storage` (AWS)
- **Deployment**: HTTP server (local/Docker) or AWS Lambda
- **No dependencies** for the frontend — pure vanilla HTML/CSS/JS, no npm, no build tools

## Key Files

| File | Purpose |
|---|---|
| `main.go` | Entry point, runtime detection (Lambda vs HTTP server) |
| `handlers.go` | HTTP handlers + **the entire UI template** (`renderHTML`) |
| `assets.go` | Embedded favicon |
| `storage_local.go` | Local disk storage + `Storage` interface definition |
| `storage_s3.go` | AWS S3 storage |
| `lambda.go` | Lambda adapter for API Gateway v1/v2 |
| `utils.go` | Note ID generation/validation, HTML escaping, ClientIP |
| `*_test.go` | Unit tests |

## Important Conventions

1. **All UI lives in `handlers.go`** — the `renderHTML` function contains the full HTML document as a Go string literal. There are no separate `.html`, `.css`, or `.js` files.
2. **Dynamic values** are injected via Go string concatenation with `EscapeHTML()` for XSS safety.
3. **Tests must pass**: Run `go test -v ./...` before committing changes.
4. **No new dependencies** for the frontend. Keep it vanilla.
5. **Responsive design** — must work on both desktop and mobile.
6. **Theme**: White background with blue accents (`#2563EB` primary, `#1D4ED8` hover).

## Running Locally

```bash
export PATH=/usr/local/go/bin:$PATH
NOTE_DIR=/tmp/note PORT=8000 go run .
```

## Testing

```bash
export PATH=/usr/local/go/bin:$PATH
go test -v ./...
```
