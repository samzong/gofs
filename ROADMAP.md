# GOFS Simple Roadmap

_Last updated: 2025-07-31_

## Quick Wins

### 1. Basic Auth (Must have)

- [x] Add simple HTTP Basic Auth (username/password)
    - [x] Add the parameter `--auth username:password` (-a) to specify the user name and password, in the format of username:password
- [x] Protect all endpoints with one switch

### 2. Health Check (Easy)

- [x] Add `/health` endpoint - returns 200 OK
- [x] Add basic metrics: uptime, version
- [x] No external dependencies

## Medium Effort

### 3. Docker Support

- [x] Simple Dockerfile (single stage, small image)
- [x] Basic docker-compose.yml for local dev
- [x] Environment variable config

### 4. Simple Metrics

- [x] Add `/metrics` endpoint with basic counters
- [x] Track: requests total, request duration, errors
- [x] Text format (Prometheus compatible)

## Later (When needed)

### 5. Multi-directory (Optional)

- [ ] Allow multiple root directories via config
- [ ] Simple path mapping
- [x] No fancy features

### 6. Cloud Storage (Future)

- [ ] Start with S3-compatible storage
- [ ] Simple interface, one backend at a time
- [x] Keep local storage as default
