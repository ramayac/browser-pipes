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

mock-msg-markdown: build build-mocks
	@echo "📨 Sending mock markdown request to Plumber..."
	@$(MAKE) mock-msg MSG='{"url":"https://en.wikipedia.org/wiki/Pipil_people", "target":"markdown", "timestamp": 1679800000}'

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
