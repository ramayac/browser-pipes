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
| `all` | Builds plumber, mocker, and all tools. | `make all` |
| `build` | Compiles the `plumber` binary into `bin/`. | `make build` |
| `build-mocks` | Compiles the `mocker` tool for testing. | `make build-mocks` |
| `build-tools` | Compiles helper tools (`go-read-md`). | `make build-tools` |
| `clean` | Removes the `bin/` directory and built binaries. | `make clean` |
| `validate-config` | Validates the plumber configuration file. | `make validate-config [CONFIG=path]` |
| `test-config` | Tests plumber with mock native messaging input. | `make test-config [MSG=...] [CONFIG=...]` |
| `test-read-md` | Tests the markdown extraction tool. | `make test-read-md [URL=...] [OUTPUT=...]` |
| `install-config` | Creates config directory and installs default `plumber.yaml`. | `make install-config` |
| `install-host` | Registers plumber as a native messaging host. | `make install-host EXTENSION_ID=...` |
| `uninstall-host` | Removes native messaging host registration. | `make uninstall-host` |

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
  open_browser:
    parameters:
      browser:
        type: string
        default: "google-chrome"
    steps:
      - run: "<<parameters.browser>> '{url}'"

  save_markdown:
    parameters:
      output_dir:
        type: string
        default: "~/Documents/ReadLater"
    steps:
      - run: "go-read-md --output <<parameters.output_dir>> '{url}'"

jobs:
  default_firefox:
    steps:
      - open_browser:
          browser: "firefox"

  read_markdown:
    steps:
      - save_markdown

workflows:
  smart_routing:
    jobs:
      - read_markdown:
          match: "(?i)(medium\\.com)"
      - default_firefox:
          match: ".*"
```

See [plumber.example.yaml](./plumber.example.yaml) for a complete working example.

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

## 🤖 Contributing

For AI agents and developers: See [AGENT.md](./AGENT.md) for development guidelines and workflow.

---

## 📄 License
MIT
