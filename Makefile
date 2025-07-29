# Go Clean Architecture Makefile

# Variables
BINARY_NAME=app
MAIN_FILE=./cmd/main.go
SWAGGER_OUTPUT=docs

# Default target
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make dev      - Run the application in development mode"
	@echo "  make build    - Build the application"
	@echo "  make swagger  - Initialize Swagger documentation"
	@echo "  make clean    - Clean build artifacts"
	@echo "  make help     - Show this help message"

# Development mode - run with hot reload
.PHONY: dev
dev:
	@echo "🚀 Starting development server..."
	@if command -v air > /dev/null; then \
		echo "Using Air for hot reload..."; \
		air; \
	else \
		echo "Air not found, running with go run..."; \
		go run $(MAIN_FILE); \
	fi

# Build the application
.PHONY: build
build:
	@echo "🔨 Building application..."
	@CGO_ENABLED=0 go build -ldflags "-s -w" -o $(BINARY_NAME) $(MAIN_FILE)
	@echo "✅ Build completed: $(BINARY_NAME)"

# Initialize Swagger documentation
.PHONY: swagger
swagger:
	@echo "📚 Initializing Swagger documentation..."
	@if command -v swag > /dev/null; then \
		swag init --parseDependency --parseInternal -g $(MAIN_FILE) -o $(SWAGGER_OUTPUT); \
		echo "✅ Swagger documentation generated in $(SWAGGER_OUTPUT)/"; \
	else \
		echo "❌ Swag CLI not found. Please install it first:"; \
		echo "   go install github.com/swaggo/swag/cmd/swag@latest"; \
		exit 1; \
	fi

# Clean build artifacts
.PHONY: clean
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@rm -rf $(SWAGGER_OUTPUT)
	@echo "✅ Clean completed"

# Install development dependencies
.PHONY: install-deps
install-deps:
	@echo "📦 Installing development dependencies..."
	@go install github.com/swaggo/swag/cmd/swag@latest
	@go install github.com/cosmtrek/air@latest
	@echo "✅ Dependencies installed"

# Run tests
.PHONY: test
test:
	@echo "🧪 Running tests..."
	@go test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "🧪 Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"

# Lint the code
.PHONY: lint
lint:
	@echo "🔍 Linting code..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "❌ golangci-lint not found. Please install it first."; \
		exit 1; \
	fi

# Format code
.PHONY: fmt
fmt:
	@echo "🎨 Formatting code..."
	@go fmt ./...
	@echo "✅ Code formatted"

# Generate mock files (if using mockgen)
.PHONY: mocks
mocks:
	@echo "🎭 Generating mocks..."
	@if command -v mockgen > /dev/null; then \
		echo "Generating mocks..."; \
		mockgen -source=internal/domain/repository.go -destination=internal/mocks/repository_mock.go; \
	else \
		echo "❌ mockgen not found. Please install it first:"; \
		echo "   go install github.com/golang/mock/mockgen@latest"; \
		exit 1; \
	fi

# All-in-one development setup
.PHONY: setup
setup: install-deps swagger
	@echo "✅ Development environment setup completed!"
	@echo "Run 'make dev' to start the development server"
