# Makefile for Go Application

# Application name
APP_NAME := ImageDownloader

# Output directory for binaries
OUTPUT_DIR := bin

# Target architecture auto-detection
ifndef GOARCH
  GOARCH := $(shell dpkg --print-architecture 2>/dev/null || rpm --eval %{_arch} 2>/dev/null || echo "unknown")
  GOARCH := $(if $(filter unknown,$(GOARCH)),$(shell uname -m),$(GOARCH))
  GOARCH := $(if $(filter x86_64,$(GOARCH)),amd64,$(GOARCH))
  GOARCH := $(if $(filter unknown,$(GOARCH)),arm64,$(GOARCH)) #default
  GOARCH := $(strip $(GOARCH))
endif

# Optional: Versioning (you can set this dynamically)
VERSION=$(shell \
  latest_tag=$$(git describe --tags --abbrev=0); \
  latest_commit=$$(git rev-parse --short HEAD); \
  if git describe --tags --exact-match HEAD >/dev/null 2>&1; then \
    echo "$$latest_tag"; \
  else \
    echo "$$latest_tag-$$latest_commit\_dev"; \
  fi)

OS_NAME=$$(uname -s)

# Define the main Go file
MAIN_FILE := main.go

# Ensure the OUTPUT_DIR exists

# Phony targets to ensure Make doesn't confuse them with files
.PHONY: all build build-linux build-mac build-windows build-all clean prepare help format lint recreate-mod test info build-docker-windows build-docker-linux

help:
	@echo "Makefile for $(APP_NAME) - Go application"
	@echo "Usage: make [target]"
	@echo "Targets:"
	@echo "Main targets:"
	@echo "  build       : Build the application for the current architecture"
	@echo "  build-all   : Build the application for all supported platforms"
	@echo "  clean       : Remove build artifacts"
	@echo ""
	@echo "Specific Architecture Build Targets:"
	@echo "  build-windows : Build Windows binary"
	@echo "  build-linux   : Build Linux binary"
	@echo "  build-mac     : Build macOS binary (Not tested)"
	@echo ""
	@echo "Docker Build Targets:"
	@echo "(simple building executables inside docker containers for Lin and Win)"
	@echo "  build-docker-windows : Build Windows binary using Docker"
	@echo "  build-docker-linux   : Build Linux binary using Docker"
	@echo ""
	@echo "Helpers:"
	@echo "  prepare     : Download and install dependencies"
	@echo "  format      : Format the source code"
	@echo "  lint        : Lint the source code"
	@echo "  test        : Run tests"
	@echo "  recreate-mod: Recreate go.mod and go.sum files"
	@echo "  info        : Show env variables"
	@echo ""
	@echo "Environment Variables:"
	@echo "  APP_NAME   : The name of the application (default: GoGasSimulator)"
	@echo "  VERSION    : The version of the application based on Git tags and commit hash"
	@echo "  GOARCH     : Target architecture for the build (default: autodetect)"
	@echo "  OS_NAME    : Detected operating system used to choose the build target"
	@echo "  OUTPUT_DIR : Directory where the compiled binaries are stored (default: bin)"
	@echo ""
	@echo "Current values:"
	@echo "APP_NAME:   $(APP_NAME)"
	@echo "VERSION:    $(VERSION)"
	@echo "GOARCH:     $(GOARCH)"
	@echo "OS_NAME:    $(OS_NAME)"
	@echo "OUTPUT_DIR: $(OUTPUT_DIR)"
	@echo ""
	@echo "Hint: You can set the GOARCH variable when running make, e.g.,"
	@echo "      make build GOARCH=amd64"

all: prepare build-linux #build-windows build-mac

# Preparation step to download dependencies
prepare:
	@echo "Downloading dependencies..."
	@go mod download
	@echo "Dependencies downloaded."

# Build for the current architecture
build: prepare
	@echo "Current OS ($(OS_NAME)) architecture ($(GOARCH))..."
	@OS_NAME=$$(uname -s); \
	if [ "$$OS_NAME" = "Linux" ]; then \
            $(MAKE) build-linux; \
        elif [ "$$OS_NAME" = "Darwin" ]; then \
            $(MAKE) build-mac; \
        elif [ "$$OS_NAME" = "MINGW64_NT" ] || [ "$$OS_NAME" = "MSYS_NT" ]; then \
            $(MAKE) build-windows; \
        else \
            echo "Unsupported operating system: $$OS_NAME"; \
            exit 1; \
        fi

# Build for Linux
build-linux: prepare
	@echo "Building for Linux..."
	@GOOS=linux GOARCH=$(GOARCH) go build -ldflags "-X main.Version=$(VERSION)" -o $(OUTPUT_DIR)/$(APP_NAME)-$(VERSION)_linux_$(GOARCH) $(MAIN_FILE)
	@echo "Linux build completed: $(OUTPUT_DIR)/$(APP_NAME)-$(VERSION)_linux_$(GOARCH)"

# Build for macOS (Not tested)
build-mac: prepare
	@echo "Building for macOS (not tested)..."
	@GOOS=darwin GOARCH=$(GOARCH) go build -ldflags "-X main.Version=$(VERSION)" -o $(OUTPUT_DIR)/$(APP_NAME)-$(VERSION)_mac_$(GOARCH) $(MAIN_FILE)
	@echo "macOS build completed: $(OUTPUT_DIR)/$(APP_NAME)-$(VERSION)_mac_$(GOARCH)"

# Build for Windows
build-windows: prepare
	@echo "Building for Windows..."
	@GOOS=windows GOARCH=$(GOARCH) go build -ldflags "-X main.Version=$(VERSION)" -o $(OUTPUT_DIR)/$(APP_NAME)-$(VERSION)_windows_$(GOARCH).exe $(MAIN_FILE)
	@echo "Windows build completed: $(OUTPUT_DIR)/$(APP_NAME)-$(VERSION)_windows_$(GOARCH).exe"

build-docker-windows:
	@echo "Building Windows binary in Docker..."
	@DOCKER_BUILDKIT=1 docker build --build-arg APP_NAME=$(APP_NAME)-$(VERSION)_windows_$(GOARCH) -f Dockerfile.windows --output type=local,dest=./$(OUTPUT_DIR) .
	@echo "Windows Docker build completed: $(OUTPUT_DIR)/$(APP_NAME)-$(VERSION)_windows_$(GOARCH).exe"

build-docker-linux:
	@echo "Building Linux binary in Docker..."
	@DOCKER_BUILDKIT=1 docker build --build-arg APP_NAME=$(APP_NAME)-$(VERSION)_linux_$(GOARCH) -f Dockerfile.linux --output type=local,dest=./$(OUTPUT_DIR) .
	@echo "Linux Docker build completed: $(OUTPUT_DIR)/$(APP_NAME)-$(VERSION)_linux_$(GOARCH)"

# Build for all platforms
build-all: prepare build-linux build-mac build-windows
	@echo "All builds completed successfully!"

# Clean compiled binaries
clean:
	@echo "Cleaning up..."
	@rm -rf $(OUTPUT_DIR)
	@echo "Cleaned."

# Recreate go.mod and go.sum files
recreate-mod:
	@echo "Recreating go.mod and go.sum files..."
	@rm -f go.mod go.sum
	@go mod init your-module-name
	@go mod tidy
	@echo "go.mod and go.sum have been recreated successfully."

format:
	gofmt -s -w ./

lint:
	golint

test:
	go test -v

info:
	@echo "Environment Variables:"
	@echo "APP_NAME:   $(APP_NAME)"
	@echo "VERSION:    $(VERSION)"
	@echo "GOARCH:     $(GOARCH)"
	@echo "OS_NAME:    $(OS_NAME)"
	@echo "OUTPUT_DIR: $(OUTPUT_DIR)"
	@echo "FILENAME:   $(APP_NAME)-$(VERSION)_$(OS_NAME)_$(GOARCH)"
	@echo ""
	@echo "Hint: You can override any variable when running make, e.g.,"
	@echo "      make build GOARCH=arm64"

