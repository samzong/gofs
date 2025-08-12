# gofs

A lightweight, fast HTTP file server written in Go.

[![Go Report Card](https://goreportcard.com/badge/github.com/samzong/gofs)](https://goreportcard.com/report/github.com/samzong/gofs)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/samzong/gofs)](https://hub.docker.com/r/samzong/gofs)

## Why

- Fast and safe: secure path handling, optional Basic Auth (bcrypt)
- Simple: one binary, zero config to start
- Modern UI: advanced theme with upload, folder creation, and ZIP downloads
- WebDAV: optional /dav/ endpoint (read-only)
- Production‑ready: structured logs, health probes, graceful shutdown
- Multiple directories: mount multiple paths with optional read-only flags

## Screenshot

![Screenshot](./screenshot.png)

## Install

Homebrew
```bash
brew tap samzong/tap
brew install gofs
```

CLI
```bash
go install github.com/samzong/gofs/cmd/gofs@latest
```

## Quick start

```bash
# Serve current directory at http://127.0.0.1:8000
gofs

# Change host/port
gofs -host 0.0.0.0 -port 3000

# Enable auth
gofs -auth user:password

# Modern UI (upload, create folder)
gofs --theme advanced

# WebDAV on /dav (read‑only by default)
gofs --enable-webdav
```

## Mounting directories

You can expose one or more directories. Format: [path:]dir[:ro][:name]

```bash
# Single dir (default is ".")
gofs -d /srv/files

# Multiple mounts with names and read‑only flags
gofs -d "/data:/srv:ro:Data" -d "/logs:/var/log::Logs"
```

## JSON API

Every listing can be JSON by sending: Accept: application/json

```bash
curl -H "Accept: application/json" http://localhost:8000/
```

## Health checks

- HTTP: /healthz and /readyz (200 OK)
- CLI: gofs --health-check

## Configuration

Flags have GOFS\_\* env twins (flags win):

- GOFS_PORT, GOFS_HOST, GOFS_DIR (semicolon‑separated for multiple)
- GOFS_THEME, GOFS_SHOW_HIDDEN, GOFS_AUTH, GOFS_ENABLE_WEBDAV
- GOFS_LOG_LEVEL (debug/info/warn/error), GOFS_ENV (production for JSON logs)

## Requirements

Go 1.24+

## Examples

See examples/ for Docker and Kubernetes manifests.

## License

MIT. See [LICENSE](LICENSE).
