.PHONY: all fmt lint test build clean security install

all: lint test build

fmt:
	goimports -w .

lint:
	golangci-lint run

test:
	go test -race -coverprofile=coverage.out ./...

build:
	go build -o kt ./cmd/kt

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
