# Contributing to terraform-provider-ebhelper

Thank you for your interest in contributing! This document provides guidelines and instructions for contributing.

## Getting Started

1. Fork the repository
2. Clone your fork locally
3. Create a feature branch from `main`
4. Make your changes
5. Run tests to ensure nothing is broken
6. Submit a pull request

## Development Setup

### Prerequisites

- Go 1.22+ (or use [mise](https://mise.jdx.dev/) which manages the Go version automatically)
- Terraform CLI (for manual testing)

### Building

```bash
make build
```

### Running Tests

```bash
make test
```

### Installing Locally

```bash
make install
```

This places the binary in `~/.terraform.d/plugins/` so Terraform can find it.

## Code Guidelines

- Follow standard Go conventions (`gofmt`, `go vet`)
- All exported types and functions must have doc comments
- Use the existing code patterns as a reference
- Add unit tests for new functionality
- Keep error messages descriptive and prefixed with the resource name

## Testing

### Unit Tests

Unit tests use mocked AWS interfaces. Run them with:

```bash
go test ./... -v -count=1
```

### Acceptance Tests

Acceptance tests require real AWS credentials and will create/destroy resources:

```bash
make testacc
```

These are not run in CI by default.

## Pull Request Process

1. Update documentation if you're changing public-facing behavior
2. Add or update tests to cover your changes
3. Ensure `go vet ./...` passes cleanly
4. Ensure `go test ./... -count=1` passes
5. Keep commits focused and well-described
6. Reference any related issues in your PR description

## Reporting Issues

- Use GitHub Issues for bug reports and feature requests
- Include Terraform version, provider version, and relevant configuration
- For bugs, include expected vs actual behavior and any error messages

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
