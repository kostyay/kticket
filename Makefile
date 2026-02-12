.PHONY: all fmt lint test build clean security install

VERSION := $(shell git describe --tags --always --dirty)
COMMIT  := $(shell git rev-parse --short HEAD)
DATE    := $(shell date -u +%FT%TZ)
LDFLAGS := -s -w \
  -X 'github.com/kostyay/kticket/internal/version.Version=$(VERSION)' \
  -X 'github.com/kostyay/kticket/internal/version.Commit=$(COMMIT)' \
  -X 'github.com/kostyay/kticket/internal/version.Date=$(DATE)'

all: lint test build

fmt:
	goimports -w .

lint:
	golangci-lint run

test:
	go test -race -coverprofile=coverage.out ./...

build:
	go build -ldflags "$(LDFLAGS)" -o kt ./cmd/kt

clean:
	rm -f kt coverage.out

security:
	@echo "Running gitleaks..."
	@command -v gitleaks >/dev/null 2>&1 && gitleaks detect --source . --verbose || echo "gitleaks not installed (brew install gitleaks)"
	@echo "Running trufflehog..."
	@command -v trufflehog >/dev/null 2>&1 && trufflehog git file://. --only-verified --fail || echo "trufflehog not installed (brew install trufflehog)"

install: build
	@echo "Installing kt to /usr/local/bin/kt (run with sudo if needed)"
	ln -sf $(CURDIR)/kt /usr/local/bin/kt
