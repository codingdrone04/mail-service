# mail-service

Contact form email microservice written in Go.

## Setup

```bash
cp .env.example .env
# Edit .env with your SMTP credentials
```

## Run

```bash
make run
```

## Build

```bash
make build
./bin/mail-service
```

## API

### `GET /health`
Health check endpoint.

### `POST /send`
Send a contact form email.

```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "subject": "Hello",
  "message": "Your message here"
}
```
