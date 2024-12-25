VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null)
NOTES ?= $(VERSION)
BINARY_NAME=foulbot
.PHONY: all tidy build

build:
	go mod tidy
	GOOS=linux GOARCH=amd64 go build -gcflags=all="-l -B -C" -ldflags "-w -s -X main.VERSION=$(VERSION)" -o $(BINARY_NAME)-linux-amd64 main.go
	GOOS=darwin GOARCH=amd64 go build -gcflags=all="-l -B -C" -ldflags "-w -s -X main.VERSION=$(VERSION)" -o $(BINARY_NAME)-darwin-amd64 main.go
	GOOS=windows GOARCH=amd64 go build -gcflags=all="-l -B -C" -ldflags "-w -s -H windowsgui -X main.VERSION=$(VERSION)" -o $(BINARY_NAME)-windows-amd64.exe main.go
	GOOS=linux GOARCH=arm64 go build -gcflags=all="-l -B -C" -ldflags "-w -s -X main.VERSION=$(VERSION)" -o $(BINARY_NAME)-linux-arm64 main.go
	GOOS=darwin GOARCH=arm64 go build -gcflags=all="-l -B -C" -ldflags "-w -s -X main.VERSION=$(VERSION)" -o $(BINARY_NAME)-darwin-arm64 main.go
	GOOS=windows GOARCH=arm64 go build -gcflags=all="-l -B -C" -ldflags "-w -s -H windowsgui -X main.VERSION=$(VERSION)" -o $(BINARY_NAME)-windows-arm64.exe main.go

run: clean
	OS=$$(uname -s | tr '[:upper:]' '[:lower:]') ; \
	ARCH=$$(uname -m) ; \
	EXTENSION=$$(if [ $$OS = "windows" ]; then echo ".exe"; fi) ; \
	GOOS=$$OS GOARCH=$$ARCH go build -gcflags=all="-l -B -C" -ldflags "-w -s -X main.VERSION=$(VERSION)" -o $(BINARY_NAME)-$$OS-$$ARCH$$EXTENSION main.go ; \
	./$(BINARY_NAME)-$$OS-$$ARCH$$EXTENSION

clean:
	rm -f $(BINARY_NAME)*

release: build
	@echo "Creating release $(VERSION)..."
	@gh release create $(VERSION) \
		$(BINARY_NAME)-linux-amd64 \
		$(BINARY_NAME)-darwin-amd64 \
		$(BINARY_NAME)-windows-amd64.exe \
		$(BINARY_NAME)-linux-arm64 \
		$(BINARY_NAME)-darwin-arm64 \
		$(BINARY_NAME)-windows-arm64.exe \
		--title "Release $(VERSION)" \
		--notes "$(NOTES)" \
		--draft=false