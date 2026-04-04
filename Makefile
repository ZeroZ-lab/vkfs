.PHONY: build clean test install

# Build targets
build: build-vkfs build-admin

build-vkfs:
	go build -o bin/vkfs ./cmd/vkfs

build-admin:
	go build -o bin/vkfs-admin ./cmd/vkfs-admin

# Install to $GOPATH/bin
install:
	go install ./cmd/vkfs
	go install ./cmd/vkfs-admin

# Tests
test:
	go test ./pkg/... ./internal/... -v

test-unit:
	go test ./tests/unit/... -v

test-integration:
	go test ./tests/integration/... -v

test-short:
	go test ./... -short -v

# Code quality
vet:
	go vet ./...

clean:
	rm -rf bin/
