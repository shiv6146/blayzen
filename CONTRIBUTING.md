# Contributing to Blayzen

Thank you for your interest in contributing to Blayzen! We welcome contributions from everyone. This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Development Workflow](#development-workflow)
- [Code Guidelines](#code-guidelines)
- [Testing](#testing)
- [Documentation](#documentation)
- [Submitting Changes](#submitting-changes)
- [Community](#community)

## Code of Conduct

This project follows a code of conduct to ensure a welcoming environment for all contributors. By participating, you agree to:

- Be respectful and inclusive
- Focus on constructive feedback
- Accept responsibility for mistakes
- Show empathy towards other community members
- Help create a positive environment

See our [Code of Conduct](CODE_OF_CONDUCT.md) for details.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Git
- Basic understanding of Go channels and concurrency
- Familiarity with WebSocket protocols (helpful)

### Quick Setup

```bash
# Fork and clone the repository
git clone https://github.com/your-username/blayzen.git
cd blayzen

# Install dependencies
make deps

# Run tests to ensure everything works
make test

# Build the project
make build
```

## Development Setup

### 1. Fork and Clone

```bash
# Fork the repository on GitHub
# Then clone your fork
git clone https://github.com/your-username/blayzen.git
cd blayzen
```

### 2. Set Up Development Environment

```bash
# Install development dependencies
make deps

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/cosmtrek/air@latest  # For hot reloading

# Set up pre-commit hooks (optional)
pip install pre-commit
pre-commit install
```

### 3. Verify Setup

```bash
# Run all checks
make check

# Run tests with coverage
make test-cover

# Build and test examples
make build-examples
```

## Development Workflow

### 1. Choose an Issue

- Check the [issue tracker](https://github.com/shiva/blayzen/issues) for open issues
- Look for issues labeled `good first issue` or `help wanted`
- Comment on the issue to indicate you're working on it

### 2. Create a Branch

```bash
# Create a feature branch
git checkout -b feature/your-feature-name

# Or create a bug fix branch
git checkout -b fix/issue-number-description
```

### 3. Make Changes

```bash
# Make your changes
# Write tests for new functionality
# Update documentation as needed

# Run tests frequently
make test

# Run linting
make lint

# Format code
make fmt
```

### 4. Test Your Changes

```bash
# Run unit tests
go test ./...

# Run tests with race detector
make test-race

# Run integration tests
make test-integration

# Run end-to-end tests
make test-e2e

# Test examples
make run-simple
```

### 5. Update Documentation

```bash
# Update README.md if needed
# Update code comments
# Update examples
# Update API documentation
```

## Code Guidelines

### Go Style Guidelines

- Follow standard Go formatting (`go fmt`)
- Use `gofmt -s` for additional simplifications
- Follow [Effective Go](https://golang.org/doc/effective_go.html) principles
- Use `golint` and `golangci-lint` for code quality checks

### Code Structure

```
blayzen/
├── cmd/                    # Main applications
│   └── blayzen/           # Main CLI application
├── internal/              # Private application code
│   ├── frame/            # Frame data structures
│   ├── pipeline/         # Pipeline orchestration
│   └── transport/        # Transport implementations
├── pkg/                   # Public library code
│   ├── agent/            # Agent configuration
│   ├── processor/        # Processor implementations
│   └── vad/              # Voice activity detection
├── examples/             # Example applications
├── test/                 # Test utilities and E2E tests
└── docs/                 # Documentation
```

### Naming Conventions

- Use descriptive names for variables, functions, and types
- Follow Go naming conventions (exported names start with capital letters)
- Use consistent naming patterns across the codebase

### Error Handling

- Return errors instead of panicking (except for truly exceptional cases)
- Use error wrapping with `fmt.Errorf` or `errors.Wrap`
- Check for errors and handle them appropriately

### Concurrency

- Use channels and goroutines appropriately
- Avoid race conditions with proper synchronization
- Use context for cancellation and timeouts
- Document goroutine lifecycles

### Comments

- Write clear, concise comments for exported functions and types
- Use complete sentences starting with the name being described
- Keep comments up to date with code changes

### Commit Messages

Follow conventional commit format:

```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes
- `refactor`: Code refactoring
- `test`: Test additions/updates
- `chore`: Maintenance tasks

Examples:
```
feat(processor): add new VAD processor implementation

fix(transport): resolve WebSocket connection timeout issue

docs: update README with new examples

test(pipeline): add integration tests for error handling
```

## Testing

### Unit Tests

```go
func TestMyFunction(t *testing.T) {
    // Arrange
    input := "test input"
    expected := "expected output"

    // Act
    result := MyFunction(input)

    // Assert
    if result != expected {
        t.Errorf("MyFunction(%q) = %q; want %q", input, result, expected)
    }
}
```

### Table-Driven Tests

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"empty string", "", ""},
        {"single word", "hello", "HELLO"},
        {"multiple words", "hello world", "HELLO WORLD"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := MyFunction(tt.input)
            if result != tt.expected {
                t.Errorf("MyFunction(%q) = %q; want %q", tt.input, result, tt.expected)
            }
        })
    }
}
```

### Integration Tests

```go
func TestPipelineIntegration(t *testing.T) {
    // Create mock transports
    inTransport := transport.NewMockTransport()
    outTransport := transport.NewMockTransport()

    // Create processors
    processor := processor.NewMyProcessor(config)

    // Build pipeline
    pipeline, err := pipeline.NewBuilder().
        WithIncoming(inTransport).
        AddProcessor(processor).
        WithOutgoing(outTransport).
        Build()
    require.NoError(t, err)

    // Test the pipeline
    ctx := context.Background()
    go pipeline.Run(ctx)

    // Send test frame
    testFrame := frame.New("test", []byte("data"))
    inTransport.SendIn(testFrame)

    // Verify output
    outputFrame := outTransport.ReceiveOut()
    assert.NotNil(t, outputFrame)
}
```

### Benchmarks

```go
func BenchmarkMyFunction(b *testing.B) {
    input := "test input"

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        MyFunction(input)
    }
}
```

## Documentation

### Code Documentation

- Document all exported functions, types, and methods
- Use proper Go doc comments
- Include examples in documentation

```go
// ProcessFrame processes a frame through the pipeline.
// It applies all configured processors in order and returns the result.
//
// Example:
//
//	processor := NewMyProcessor(config)
//	frame := frame.New("audio", audioData)
//	result := processor.ProcessFrame(frame)
func (p *MyProcessor) ProcessFrame(f *frame.Frame) *frame.Frame {
    // implementation
}
```

### README Updates

- Keep examples current and working
- Update installation instructions
- Document new features and breaking changes
- Update performance benchmarks

### API Documentation

- Use `godoc` to generate API documentation
- Keep API documentation in sync with code
- Document breaking changes clearly

## Submitting Changes

### Pull Request Process

1. **Fork the repository** and create a feature branch
2. **Make your changes** following the guidelines above
3. **Test thoroughly** - ensure all tests pass
4. **Update documentation** as needed
5. **Run all checks**: `make check`
6. **Commit your changes** with clear commit messages
7. **Push to your fork** and create a pull request

### Pull Request Checklist

- [ ] Tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] Linting passes (`make lint`)
- [ ] Security checks pass (`make security`)
- [ ] Documentation is updated
- [ ] Commit messages follow conventions
- [ ] PR description is clear and detailed
- [ ] Related issues are referenced

### Review Process

1. **Automated Checks**: CI will run tests, linting, and security checks
2. **Code Review**: Maintainers will review your code
3. **Feedback**: Address any feedback from reviewers
4. **Approval**: Once approved, your PR will be merged

## Community

### Getting Help

- **Issues**: Use GitHub issues for bugs and feature requests
- **Discussions**: Use GitHub discussions for questions and ideas
- **Discord**: Join our community Discord for real-time chat

### Recognition

Contributors are recognized through:
- GitHub contributor statistics
- Attribution in release notes
- Special mention for significant contributions

### Maintainers

For questions about contributing or the contribution process:
- Open an issue with the `question` label
- Join our Discord community
- Email the maintainers (if contact info is available)

Thank you for contributing to Blayzen! Your help makes this project better for everyone.
