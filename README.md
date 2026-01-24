# browser-pipes 🚿

**browser-pipes** is a local Unix-style Native Messaging system that treats browser URLs as a data stream. It allows you to pipe URLs between browsers, automatically clean tracking parameters, and snapshot web pages into clean, readable local files.

## 🚀 Key Features

- **The Toggle**: Instantly switch the current URL from one browser to another (e.g., Chrome -> Firefox) with a single click.
- **The Cleaner**: Automatically strips tracking parameters like `utm_*`, `fbclid`, and `gclid` before processing.
- **The Snapshot**: Extracts the main content of a page using `go-readability` and saves it as a clean Markdown file for offline reading.
- **Rule-based Routing**: Define regex rules to automatically route specific domains to specific browsers or actions.
- **Unix-style Logging**: Monitor all activity in real-time using `tail -f` on the Plumber's stderr logs.

## 🏗️ Architecture

- **The Plumber (Go)**: A backend binary that acts as a router and processor. It communicates with browsers via the Standard Native Messaging protocol.
- **The Extension (Manifest V3)**: A lightweight browser extension that sends the current URL and metadata to the Plumber.

---

## 🛠️ Makefile Targets

| Target | Description | Usage |
| :--- | :--- | :--- |
| `all` | Builds both the `plumber` and the `mocker` tools. | `make all` |
| `build` | Compiles the `plumber` binary into `bin/`. | `make build` |
| `build-mocks` | Compiles the `mocker` tool for testing. | `make build-mocks` |
| `clean` | Removes the `bin/` directory and built binaries. | `make clean` |
| `install-config` | Creates the config directory and installs the default `plumber.yaml`. | `make install-config` |
| `mock-msg` | Sends a custom JSON message to the Plumber via the mocker. | `make mock-msg MSG='{"url":"..."}'` |
| `mock-msg-snapshot` | Sends a pre-defined snapshot request to the Plumber for testing. | `make mock-msg-snapshot` |
| `test-config-v2` | Runs the Plumber with the V2 example configuration for verification. | `make test-config-v2` |

---

## ⚙️ Setup & Configuration

### 1. Build the Project
```bash
make build
```

### 2. Configure the Plumber
Install the default configuration file:
```bash
make install-config
```
You can then edit it at `~/.config/browser-pipes/plumber.yaml`.

### 3. Configuration V2 (New)

The new configuration system (Version 2) is inspired by CircleCI, allowing for reusable commands, composed jobs, and regex-based workflow routing.

#### Example `plumber.yaml` (v2)

```yaml
version: 2

commands:
  open_specific:
    parameters:
      browser:
        type: string
        default: "google-chrome"
    steps:
      - run: "<<parameters.browser>> {url}"

jobs:
  work_browsing:
    steps:
      - open_specific:
          browser: "firefox"

### 4. CLI Tooling

The `plumber` binary now supports subcommands:

- `plumber run`: Starts the Native Messaging listener (default).
- `plumber validate`: Validates the configuration file.
- `plumber schema`: Outputs the JSON Schema for the V2 configuration (useful for IDE autocompletion).

**Configuration Schema**: [plumber.schema.json](./plumber.schema.json) (Auto-generated)

**Example: Generating Documentation**
```bash
plumber schema > plumber.schema.json
```


### 3. Install Native Messaging Host
> [!NOTE]
> Detailed installation for the Native Messaging manifest is currently a manual step. You need to create a JSON manifest file for your browser (e.g., `~/.config/google-chrome/NativeMessagingHosts/com.browser.pipe.json`) pointing to the `plumber` binary.

### 4. Load the Extension
1. Open your browser's extension page (e.g., `chrome://extensions`).
2. Enable "Developer mode".
3. Click "Load unpacked" and select the `extension/` directory.

---

## 📝 TODO List

- [ ] **Native Messaging Manifests**: Create a script or template to generate the required JSON host manifests for Chrome, Firefox, Brave, and Edge.
- [ ] **Installation Script**: Automate the registration of the Plumber as a Native Messaging host across different browsers/OSes.
- [ ] **Extension Icons**: Design and add `icon.png` (16x16, 48x48, 128x128) to the extension directory.
- [ ] **Cross-Platform Support**: Validate and improve compatibility for Windows and macOS (currently Linux-focused).
- [ ] **Advanced URL Cleaning**: Allow users to define custom tracking parameters to strip via YAML?

---

## 📄 License
MIT
