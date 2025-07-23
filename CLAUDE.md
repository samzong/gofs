# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gofs is a lightweight, fast HTTP file server written in Go, designed as a modern replacement for `python -m http.server`. It features zero external dependencies, secure path handling, and cross-platform deployment.

## Development Commands

### Core Development
```bash
# Build optimized binary with version info
make build

# Run all tests
make test

# Start development server
make dev

# Clean build artifacts
make clean
```

### Testing
```bash
# Run specific test package
go test ./internal/config/
go test ./pkg/fileutil/

# Run tests with verbose output
go test -v ./...

# Generate coverage for specific package
go test -coverprofile=coverage.out ./pkg/fileutil/
```

## Architecture Overview

### Layered Design
- **cmd/gofs/**: Application entry point with CLI flag handling and graceful shutdown
- **internal/**: Core business logic (private packages)
  - **config/**: Simple configuration management with validation
  - **server/**: HTTP server implementation with graceful shutdown
  - **handler/**: File serving handlers with HTML/JSON dual responses
  - **filesystem/**: Interface-based file system abstraction
  - **types.go**: Core interfaces (FileSystem, FileInfo, APIError)
- **pkg/**: Reusable utilities (public packages)
  - **fileutil/**: Path safety, MIME detection, file utilities

### Key Patterns
- **Interface-First Design**: Core abstractions (FileSystem, FileInfo) defined in `internal/types.go`
- **Zero External Dependencies**: Built entirely with Go standard library
- **Path Safety**: All file operations use `pkg/fileutil/safe.go` to prevent traversal attacks
- **Dual Response Format**: Handlers support both HTML (browser) and JSON (API) responses

## Configuration System

Simple configuration with CLI flags and validation. Main config struct in `internal/config/config.go`:
```go
type Config struct {
    Port int    // Port to listen on (default: 8000)
    Host string // Host address to bind to (default: "0.0.0.0")
    Dir  string // Root directory to serve files from (default: ".")
}
```

Configuration validation includes:
- Port range validation (1-65535)
- Directory existence and accessibility checks
- Automatic conversion to absolute paths

## Security Considerations

- **Path Traversal Protection**: All file operations use `pkg/fileutil/safe.go` with double-validation
- **Safe Path Function**: Prevents `../` attacks by checking before and after path cleaning
- **Read-Only by Design**: Server only serves files, no upload or modification capabilities
- **Input Validation**: All file paths validated and sanitized

## Testing Guidelines

Current test files provide patterns to follow:
- `pkg/fileutil/safe_test.go`: Security-critical path handling with table-driven tests
- `pkg/fileutil/mime_test.go`: MIME type detection with comprehensive test cases

Use table-driven tests for multiple test cases. All tests currently pass with 88.6% coverage in fileutil package.

## Performance Notes

- Zero external dependencies - built with Go standard library only
- Optimized binary size with build flags (`-s -w -trimpath`)
- Goroutine-based concurrent request handling
- Fast startup time with minimal memory footprint

## Dependencies

**Zero external dependencies** - The project uses only Go standard library.

**Go version**: 1.21+ (to support modern Go features)

## Development Patterns

### Error Handling
- Use structured API errors via `internal.APIError` type
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Check handlers for dual HTML/JSON error response patterns

### File Operations
**Critical**: Always use `pkg/fileutil/safe.go` functions for any path operations to prevent directory traversal attacks.

### Adding New Features
1. Define interfaces in `internal/types.go` if needed
2. Implement in appropriate `internal/` package  
3. Update config struct in `internal/config/config.go` if configuration is needed
4. Add validation and tests
5. Wire up in main.go server initialization

### Build Process
The Makefile uses linker flags to embed version information:
```bash
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}"
```