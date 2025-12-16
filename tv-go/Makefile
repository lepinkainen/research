.PHONY: all build run clean dev prod docker

# Build the application
build:
	go build -o tv-pocketbase

# Build with optimizations for production
prod:
	go build -ldflags="-s -w" -o tv-pocketbase

# Run in development mode
dev:
	ENV=development go run .

# Run the application
run:
	./tv-pocketbase serve

# Clean build artifacts
clean:
	rm -f tv-pocketbase
	rm -rf pb_data

# Download dependencies
deps:
	go mod download
	go mod tidy

# Run tests
test:
	go test -v ./...

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/tv-pocketbase-linux-amd64
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/tv-pocketbase-linux-arm64
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/tv-pocketbase-darwin-amd64
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/tv-pocketbase-darwin-arm64
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/tv-pocketbase-windows-amd64.exe

# Docker build
docker:
	docker build -t tv-pocketbase .

# Create admin user
admin:
	./tv-pocketbase admin create

# Backup database
backup:
	./tv-pocketbase backup

# Install as systemd service (Linux only)
install-service:
	sudo cp tv-pocketbase /opt/tv-pocketbase/
	sudo cp tv-pocketbase.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable tv-pocketbase
	sudo systemctl start tv-pocketbase

# View logs (systemd)
logs:
	sudo journalctl -u tv-pocketbase -f

# Quick setup: build, create admin, run
setup: build
	./tv-pocketbase admin create
	./tv-pocketbase serve
