# Contributing to webform-service

Thanks for your interest in contributing! Here are some guidelines to help you get started.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/<your-username>/webform-service.git`
3. Create a branch: `git checkout -b my-feature`
4. Make your changes
5. Run tests: `go test -v -race ./...`
6. Run linter: `golangci-lint run`
7. Commit and push your changes
8. Open a pull request

## Development Setup

**Prerequisites:**
- Go 1.22+
- Docker (optional, for container builds)
- golangci-lint (for linting)

**Running locally:**

```bash
# Set required environment variables
export SMTP_HOST=your-smtp-host
export SMTP_USER=your-smtp-user
export SMTP_PASS=your-smtp-pass
export DEV_MODE=true

# Create a sites.json (see README for format)

# Run the service
go run ./cmd/server
```

## Guidelines

- Keep changes focused and minimal
- Write clear commit messages
- Add tests for new functionality
- Ensure all existing tests pass
- Follow existing code style (enforced by golangci-lint)

## Reporting Issues

When opening an issue, please include:
- A clear description of the problem
- Steps to reproduce
- Expected vs actual behavior
- Go version and OS

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
