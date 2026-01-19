BINARY_NAME=plumber
MOCKER_NAME=mocker
BUILD_DIR=bin

.PHONY: all build clean test mock-msg install-config

all: build build-mocks

build:
	@echo "🔧 Building Plumber..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) cmd/plumber/main.go cmd/plumber/utils.go

build-mocks:
	@echo "🔧 Building Mocker..."
	go build -o $(BUILD_DIR)/$(MOCKER_NAME) tools/mocker/main.go

clean:
	@echo "🧹 Cleaning..."
	rm -rf $(BUILD_DIR)

# Usage: make mock-msg MSG='{"url":"https://example.com"}'
mock-msg: build build-mocks
	@echo "📨 Sending mock message to Plumber..."
	@echo '$(MSG)' | $(BUILD_DIR)/$(MOCKER_NAME) | $(BUILD_DIR)/$(BINARY_NAME)

mock-msg-snapshot: build build-mocks
	@echo "📨 Sending mock snapshot to Plumber..."
	@$(MAKE) mock-msg MSG='{"url":"https://en.wikipedia.org/wiki/Pipil_people", "target":"snapshot", "timestamp": 1679800000}'

install-config:
	@echo "📦 Installing default configuration..."
	@mkdir -p $(HOME)/.config/browser-pipes
	@if [ ! -f $(HOME)/.config/browser-pipes/plumber.yaml ]; then \
		cp plumber.example.yaml $(HOME)/.config/browser-pipes/plumber.yaml; \
		echo "✅ Configuration created at ~/.config/browser-pipes/plumber.yaml"; \
	else \
		echo "⚠️ Configuration already exists at ~/.config/browser-pipes/plumber.yaml. Skipping."; \
	fi
