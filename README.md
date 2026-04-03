# webform-service

A lightweight Go microservice for handling contact form submissions. Designed for self-hosted deployments with minimal resource usage (~10MB RAM).

## Features

- **Multi-Site Support**
  - Handle contact forms for multiple domains from a single instance
  - Per-site email configuration (sender, recipient)
  - Origin-based automatic site detection

- **Spam Protection**
  - Honeypot field detection
  - Time-based validation (form must be open for 3+ seconds)
  - Per-IP rate limiting (configurable, default 3 requests/hour)
  - Input sanitization and email header injection prevention

- **Email Delivery**
  - SMTP support (works with any SMTP provider, e.g. Brevo, Mailgun, Amazon SES, self-hosted)
  - Configurable sender and recipient per site

- **Production Ready**
  - Structured JSON logging
  - Graceful shutdown
  - Health check endpoint
  - Prometheus metrics (`/metrics`)
  - CORS support (automatic per-site origin handling)
  - Optional Redis for distributed rate limiting
  - Minimal Docker image (~5-8MB)

## Quick Start

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SMTP_HOST` | Yes | - | SMTP server hostname |
| `SMTP_PORT` | No | `587` | SMTP server port |
| `SMTP_USER` | Yes | - | SMTP username/API key |
| `SMTP_PASS` | Yes | - | SMTP password |
| `PORT` | No | `8080` | HTTP server port |
| `SITES_CONFIG_PATH` | No | `./sites.json` | Path to sites configuration file |
| `RATE_LIMIT_PER_HOUR` | No | `3` | Max requests per IP per hour |
| `MIN_FORM_TIME_SECONDS` | No | `3` | Minimum seconds form must be open |
| `REDIS_URL` | No | - | Redis URL for distributed rate limiting (e.g., `redis://localhost:6379`) |
| `DEV_MODE` | No | `false` | Allow localhost origins for any site (for development) |

### Sites Configuration

The service requires a `sites.json` file that defines which sites/domains are allowed to submit contact forms. Each site has its own email configuration.

**Example `sites.json`:**
```json
{
  "sites": [
    {
      "id": "example-site",
      "origins": ["https://example.com", "https://www.example.com"],
      "recipientEmail": "contact@example.com",
      "senderEmail": "noreply@example.com",
      "senderName": "Example Contact Form"
    },
    {
      "id": "another-site",
      "origins": ["https://another.com"],
      "recipientEmail": "hello@another.com",
      "senderEmail": "noreply@another.com",
      "senderName": "Another Site Contact"
    }
  ]
}
```

**Site configuration fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique identifier for the site (used in logs and email subjects) |
| `origins` | Yes | List of allowed CORS origins for this site |
| `recipientEmail` | Yes | Email address to receive contact submissions |
| `senderEmail` | Yes | "From" email address for outgoing emails |
| `senderName` | Yes | "From" display name for outgoing emails |

### Running with Docker

```bash
docker run -d \
  -p 8080:8080 \
  -e SMTP_HOST=smtp.example.com \
  -e SMTP_USER=your-smtp-user \
  -e SMTP_PASS=your-smtp-password \
  -v /path/to/sites.json:/config/sites.json:ro \
  -e SITES_CONFIG_PATH=/config/sites.json \
  ghcr.io/m-tier/webform-service:latest
```
## API Endpoints

### POST /api/contact

Submit a contact form. The request must include an `Origin` header matching one of the configured sites.

**Request:**
```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "message": "Hello, I'd like to discuss...",
  "website": "",
  "timestamp": 1234567890
}
```

**Response (Success):**
```json
{
  "success": true,
  "message": "Message sent successfully"
}
```

**Response (Error):**
```json
{
  "success": false,
  "error": "Rate limit exceeded. Please try again later."
}
```

### GET /health

Health check endpoint.

**Response:**
```json
{
  "status": "ok"
}
```

### GET /metrics

Prometheus metrics endpoint. Exposes request counts, durations, contact submission stats, rate limit hits, and validation failures.

## SMTP Provider Examples

The service works with any SMTP provider. Here are a few common options:

| Provider | `SMTP_HOST` | `SMTP_PORT` | Notes |
|----------|-------------|-------------|-------|
| [Brevo](https://www.brevo.com/) | `smtp-relay.brevo.com` | `587` | Free tier available, use SMTP key as user/pass |
| [Mailgun](https://www.mailgun.com/) | `smtp.mailgun.org` | `587` | Use domain credentials |
| [Amazon SES](https://aws.amazon.com/ses/) | `email-smtp.<region>.amazonaws.com` | `587` | Use IAM SMTP credentials |
| Self-hosted | Your server hostname | `587` / `465` | Any standard SMTP server |

## Frontend Integration

The frontend should send a JSON POST request with the following fields:

- `name` (string, required): Sender's name
- `email` (string, required): Sender's email
- `message` (string, required): Message content (10-5000 chars)
- `website` (string, honeypot): Should be empty, hidden from users
- `timestamp` (number): Unix timestamp when the form was loaded

Example frontend implementation is available in the [taron-tech](https://github.com/M-Tier/taron-tech) repository.

## Development

```bash
# Run tests
go test -v ./...

# Run linter
golangci-lint run

# Build binary
go build -o server ./cmd/server

# Build Docker image
docker build -t webform-service .
```


## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
