BINARY_NAME=plumber
MOCKER_NAME=mocker
BUILD_DIR=bin
CONFIG?=plumber.example.yaml

.PHONY: all build clean test mock-msg install-config test-read-md

all: build build-mocks build-tools

build:
	@echo "🔧 Building Plumber..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/plumber

build-mocks:
	@echo "🔧 Building Mocker..."
	go build -o $(BUILD_DIR)/$(MOCKER_NAME) tools/mocker/main.go

build-tools:
	@echo "🔧 Building go-read-md..."
	go build -o $(BUILD_DIR)/go-read-md ./cmd/go-read-md
	@echo "🔧 Building url-hash..."
	go build -o $(BUILD_DIR)/url-hash ./cmd/url-hash

clean:
	@echo "🧹 Cleaning..."
	rm -rf $(BUILD_DIR)

test:
	@echo "🧪 Running unit tests..."
	go test -v ./cmd/...

# Usage: make mock-msg MSG='{"url":"https://example.com"}' CONFIG=...
mock-msg: build build-mocks
	@echo "📨 Sending mock message to Plumber (config: $(CONFIG))..."
	@msg='$(MSG)'; \
	if [ -z "$$msg" ]; then \
		msg='{"url":"https://example.com","target":"","timestamp":1679800000}'; \
	fi; \
	echo "$$msg" | $(BUILD_DIR)/$(MOCKER_NAME) | $(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG) run

# Demonstrate functionality with a preset example
demo: build build-mocks
	@echo "🚀 Running demo with Wikipedia example..."
	@$(MAKE) mock-msg MSG='{"url":"https://en.wikipedia.org/wiki/Pipil_people", "target":"markdown", "timestamp": 1679800000}'

# Usage: make validate-config CONFIG=...
validate-config: build
	@echo "🔍 Validating config: $(CONFIG)"
	@$(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG) validate

# Usage: make test-config MSG='{"url":"https://example.com"}' CONFIG=...
test-config: build build-mocks
	@echo "🧪 Testing with config: $(CONFIG)"
	@echo "📨 Sending mock message..."
	@msg='$(MSG)'; \
	if [ -z "$$msg" ]; then \
		msg='{"url":"https://nifmuhammad.medium.com/115-favorite-albums-of-2025-this-time-with-a-short-essay-about-brian-wilson-e12b04ee9e45","target":"","timestamp":1679800000}'; \
	fi; \
	echo "$$msg" | $(BUILD_DIR)/$(MOCKER_NAME) | $(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG) run

# Usage: make test-read-md [URL=https://example.com] [OUTPUT=/tmp/test-articles]
test-read-md: build-tools
	@echo "📝 Testing go-read-md..."
	@url='$(URL)'; \
	if [ -z "$$url" ]; then \
		url='https://example.com'; \
	fi; \
	output='$(OUTPUT)'; \
	if [ -z "$$output" ]; then \
		output='/tmp/browser-pipes-test'; \
	fi; \
	echo "   URL: $$url"; \
	echo "   Output: $$output"; \
	$(BUILD_DIR)/go-read-md --output "$$output" --verbose "$$url"
	@echo "✅ Test complete. Check the output directory for results."


install-config:
	@echo "📦 Installing default configuration..."
	@mkdir -p $(HOME)/.config/browser-pipes
	@if [ ! -f $(HOME)/.config/browser-pipes/plumber.yaml ]; then \
		cp plumber.example.yaml $(HOME)/.config/browser-pipes/plumber.yaml; \
		echo "✅ Configuration created at ~/.config/browser-pipes/plumber.yaml"; \
	else \
		echo "⚠️ Configuration already exists at ~/.config/browser-pipes/plumber.yaml. Skipping."; \
	fi

# Usage: make install-host EXTENSION_ID=your_extension_id_from_chrome
install-host: build
	@if [ -z "$(EXTENSION_ID)" ]; then \
		echo "❌ EXTENSION_ID is required (from chrome://extensions). Usage: make install-host EXTENSION_ID=..."; \
		exit 1; \
	fi
	@echo "🔌 Installing Native Messaging Host for ID: $(EXTENSION_ID)..."
	@# Create the manifest file in the build directory first to avoid shell quoting issues in the loop
	@printf '{\n  "name": "com.github.browser_pipe",\n  "description": "Browser Pipes Plumber",\n  "path": "%s/%s/%s",\n  "type": "stdio",\n  "allowed_origins": [\n    "chrome-extension://%s/"\n  ]\n}' "$(shell pwd)" "$(BUILD_DIR)" "$(BINARY_NAME)" "$(EXTENSION_ID)" > $(BUILD_DIR)/com.github.browser_pipe.json
	@for browser in "google-chrome" "chromium" "BraveSoftware/Brave-Browser" "microsoft-edge"; do \
		if [ -d "$(HOME)/.config/$$browser" ]; then \
			mkdir -p "$(HOME)/.config/$$browser/NativeMessagingHosts"; \
			cp $(BUILD_DIR)/com.github.browser_pipe.json "$(HOME)/.config/$$browser/NativeMessagingHosts/com.github.browser_pipe.json"; \
			echo "✅ Registered for $$browser"; \
		fi \
	done

uninstall-host:
	@echo "🗑️ Removing Native Messaging Host..."
	@for browser in "google-chrome" "chromium" "BraveSoftware/Brave-Browser" "microsoft-edge"; do \
		rm -f "$(HOME)/.config/$$browser/NativeMessagingHosts/com.github.browser_pipe.json"; \
	done
	@echo "✅ Removed."
