# kt version

## Summary

Add `kt version` command. Version derived from git tags at build time via `-ldflags`.

## Vars

Package: `internal/version/version.go`

- `Version` — `git describe --tags --always --dirty`
- `Commit` — short SHA
- `Date` — build timestamp (UTC)

Fallback when built without ldflags: `"dev"`.

## Command

`kt version` output:
```
kt v0.1.0-3-gabcdef (commit: abcdef, built: 2026-02-12T10:00:00Z)
```

`kt version --json`:
```json
{"version":"v0.1.0-3-gabcdef","commit":"abcdef","date":"2026-02-12T10:00:00Z"}
```

## Makefile

```makefile
VERSION := $(shell git describe --tags --always --dirty)
COMMIT  := $(shell git rev-parse --short HEAD)
DATE    := $(shell date -u +%FT%TZ)
LDFLAGS := -s -w \
  -X 'github.com/kostyay/kticket/internal/version.Version=$(VERSION)' \
  -X 'github.com/kostyay/kticket/internal/version.Commit=$(COMMIT)' \
  -X 'github.com/kostyay/kticket/internal/version.Date=$(DATE)'

build:
	go build -ldflags "$(LDFLAGS)" -o kt ./cmd/kt
```
