# gofs

A lightweight, fast HTTP file server written in Go.

[![Go Report Card](https://goreportcard.com/badge/github.com/samzong/gofs)](https://goreportcard.com/report/github.com/samzong/gofs)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Features

- **Zero dependencies**: Built with Go standard library only
- **Fast**: Optimized for performance with minimal overhead
- **Secure**: Safe path handling prevents directory traversal attacks
- **Simple**: Single binary, easy deployment
- **Cross-platform**: Works on Linux, macOS, Windows

## Quick Start

```bash
# Install
go install github.com/samzong/gofs/cmd/gofs@latest

# Serve current directory on port 8000
gofs

# Serve specific directory on custom port
gofs -port 3000 -dir /path/to/files
```

## Usage

```
Usage: gofs [options]

Options:
  -port int    Server port (default 8000)
  -host string Server host (default "0.0.0.0")  
  -dir string  Root directory to serve (default ".")
  -help        Show help
  -version     Show version
```

## API

The server provides both HTML and JSON responses:

- **HTML**: Default browser view with file listing
- **JSON**: Add `Accept: application/json` header for programmatic access

## Development

```bash
# Clone repository
git clone https://github.com/samzong/gofs.git
cd gofs

# Build
make build

# Run tests
make test

# Run with coverage
make coverage

# Start development server
make dev
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.