# Mail Service

Lightweight contact form email microservice written in Go. Receives form submissions via REST API and sends emails through SMTP.

## Features

- SMTP email sending (STARTTLS)
- HTML + plain text email templates
- Honeypot anti-bot protection
- Per-IP rate limiting
- CORS middleware
- Structured JSON logging
- Graceful shutdown

## API

### `GET /health`

Health check.

```json
{ "status": "ok", "timestamp": "2026-02-17T14:00:00Z" }
```

### `POST /send`

Send a contact form email.

**Request:**

```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "subject": "Hello",
  "message": "Your message here",
  "company": ""
}
```

| Field | Required | Max length | Notes |
|-------|----------|------------|-------|
| `name` | Yes | 100 | |
| `email` | Yes | 254 | Must be valid email |
| `subject` | No | 200 | |
| `message` | Yes | 5000 | |
| `company` | No | - | Honeypot field (must be empty) |

**Response:**

```json
{ "success": true, "message": "Message sent successfully" }
```

## Setup

```bash
cp .env.example .env
# Edit .env with your SMTP credentials
```

### Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SERVER_PORT` | No | 8080 | Server port |
| `CORS_ALLOWED_ORIGINS` | Yes | - | Comma-separated allowed origins |
| `SMTP_HOST` | Yes | - | SMTP server hostname |
| `SMTP_PORT` | No | 587 | SMTP server port |
| `SMTP_USER` | Yes | - | SMTP username |
| `SMTP_PASSWORD` | Yes | - | SMTP password / token |
| `MAIL_FROM` | Yes | - | Sender email address |
| `MAIL_TO` | Yes | - | Recipient email address |
| `RATE_LIMIT_PER_MINUTE` | No | 5 | Max requests per IP per minute |

## Run locally

```bash
make run
```

## Build

```bash
make build
./bin/mail-service
```

## Docker

```bash
docker compose up -d --build
```

## Deploy

Deployed automatically on push to `main` via GitHub Actions self-hosted runner.

```
push to main → GitHub Actions → lilypad (self-hosted runner)
  → git pull + docker compose up -d --build
```

## Tech stack

- Go 1.25
- `net/smtp` (standard library)
- `net/http` (standard library)
- Docker (Alpine)
