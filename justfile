# Build the messages binary
build:
    go build -tags goolm ./cmd/messages

# Run all tests
test:
    go test -tags goolm ./...

# Install the binary
install:
    go install -tags goolm ./cmd/messages

# Build with Nix
nix-build:
    nix build

# Format code
fmt:
    go fmt ./...

# Vet code
vet:
    go vet ./...

# Clean build artifacts
clean:
    go clean
