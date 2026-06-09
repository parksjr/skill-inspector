BINARY  := skill-inspector
MODULE  := github.com/parksjr/skill-inspector
LDFLAGS := -ldflags "-s -w"
TRIM    := -trimpath

.PHONY: build release clean vet

# Build for the current platform
build:
	go build $(LDFLAGS) $(TRIM) -o $(BINARY) .

# Cross-compile all release targets
release: clean-dist
	mkdir -p dist
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) $(TRIM) -o dist/$(BINARY)-darwin-amd64  .
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) $(TRIM) -o dist/$(BINARY)-darwin-arm64  .
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) $(TRIM) -o dist/$(BINARY)-linux-amd64   .
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) $(TRIM) -o dist/$(BINARY)-linux-arm64   .
	@echo ""
	@echo "Release binaries:"
	@ls -lh dist/

# Run static analysis
vet:
	go vet ./...

# Remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/

clean-dist:
	rm -rf dist/
