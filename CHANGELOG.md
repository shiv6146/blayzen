# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release preparation
- Comprehensive CI/CD pipeline with GitHub Actions
- Security scanning and vulnerability checks
- Code quality tools (golangci-lint, gosec)
- GitHub issue and pull request templates
- Contributing guidelines and code of conduct
- Security policy for responsible disclosure
- Dependency management with Dependabot

### Changed
- Code formatting applied across all files

### Fixed
- Various code formatting and import issues

## [0.1.0] - 2025-01-XX

### Added
- **Core Framework**: CSP-based voice agent framework with sub-200ms latencies
- **Pipeline Architecture**: Modular pipeline system for processing voice frames
- **Transport Layer**: WebSocket transport with Exotel protocol support
- **Processor System**: Pluggable processor architecture
  - VAD (Voice Activity Detection) processor
  - Turn detection processor
  - Transform processor for custom frame processing
  - Filter processor for conditional frame filtering
- **Frame System**: Unified data structure for audio, text, and metadata
- **Mock Transport**: Testing utilities for unit and integration tests
- **CLI Application**: Command-line interface for running voice bots
- **Examples**: Multiple example implementations
  - Simple echo bot
  - Advanced voice bot with configuration
  - Exotel WebSocket integration
  - Mock server for testing
- **Testing Framework**: Comprehensive test suite with E2E tests
- **Documentation**: Extensive README with usage examples and API reference

### Features
- **Low Latency**: Sub-200ms end-to-end voice processing
- **High Throughput**: Support for 1000+ concurrent streams
- **Memory Efficient**: Frame pooling and optimized memory usage (<50MB for 10K frames)
- **Production Ready**: Graceful shutdown, metrics collection, and observability
- **Developer Friendly**: Simple 3-line setup for basic voice bots

### Technical Details
- **Language**: Go 1.21+
- **Architecture**: CSP (Communicating Sequential Processes) model
- **Dependencies**: gorilla/websocket, testify, uuid
- **License**: MIT

---

## Types of changes
- `Added` for new features
- `Changed` for changes in existing functionality
- `Deprecated` for soon-to-be removed features
- `Removed` for now removed features
- `Fixed` for any bug fixes
- `Security` in case of vulnerabilities

## Versioning
This project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version for incompatible API changes
- **MINOR** version for backwards-compatible functionality additions
- **PATCH** version for backwards-compatible bug fixes

---

**Legend:**
- 🚀 New feature
- 🐛 Bug fix
- 📚 Documentation
- 🔧 Maintenance
- 💥 Breaking change
