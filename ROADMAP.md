# GOFS Simple Roadmap

_Last updated: 2025-07-29_

## Quick Wins

### 1. Basic Auth (Must have)

- [ ] Add simple HTTP Basic Auth (username/password in config)
- [ ] Protect all endpoints with one switch
- [ ] Config file format with YAML

### 2. Health Check (Easy)

- [ ] Add `/health` endpoint - returns 200 OK
- [ ] Add basic metrics: uptime, version
- [ ] No external dependencies

## Medium Effort

### 3. Docker Support

- [ ] Simple Dockerfile (single stage, small image)
- [ ] Basic docker-compose.yml for local dev
- [ ] Environment variable config

### 4. Simple Metrics

- [ ] Add `/metrics` endpoint with basic counters
- [ ] Track: requests total, request duration, errors
- [ ] Text format (Prometheus compatible)

## Later (When needed)

### 5. Multi-directory (Optional)

- [ ] Allow multiple root directories via config
- [ ] Simple path mapping
- [ ] No fancy features

### 6. Cloud Storage (Future)

- [ ] Start with S3-compatible storage
- [ ] Simple interface, one backend at a time
- [ ] Keep local storage as default
