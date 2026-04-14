.PHONY: build test test-integration lint clean

build:
	go build -o bin/mcp-jira ./cmd/server
	go build -o bin/mcp-jira-index ./cmd/index

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping"

clean:
	rm -rf bin/ server index
