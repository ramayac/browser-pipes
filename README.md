# browser-pipes 🚿

![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/ramayac/browser-pipes/coverage-badge/coverage.json)

**browser-pipes** is a local Unix-style Native Messaging system that treats browser URLs as a data stream. It allows you to pipe URLs between browsers, automatically clean tracking parameters, and snapshot web pages into clean, readable local files.

## 🚀 Key Features

- **The Toggle**: Instantly switch the current URL from one browser to another (e.g., Chrome -> Firefox) with a single click.
- **The Cleaner**: Automatically strips tracking parameters like `utm_*`, `fbclid`, and `gclid` before processing.
- **The Snapshot**: Extracts the main content of a page using `go-readability` and saves it as a clinical Markdown file. Supports multiple input modes (fetching URL directly, reading from Stdin, or reading an HTML file).
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
| `test` | Runs all unit tests. | `make test` |
| `test-coverage` | Runs tests and opens coverage report. | `make test-coverage` |
| `clean` | Removes binary files and coverage data. | `make clean` |
| `validate-config` | Validates the plumber configuration file. | `make validate-config [CONFIG=path]` |
| `test-config` | Tests plumber with mock native messaging input. | `make test-config [MSG=...] [CONFIG=...]` |
| `mock-msg` | Sends a raw JSON message to plumber via mocker. | `make mock-msg [MSG=...] [CONFIG=...]` |
| `demo` | Runs a predefined demo with a Wikipedia URL. | `make demo [CONFIG=...]` |
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

#### Job Workspaces
Every Job execution creates its own temporary workspace (CWD). This allows steps to share files and state:
- Step 1: `curl -o page.html <<parameters.url>>`
- Step 2: `go-read-md --input page.html --url <<parameters.url>>`

#### System Parameters
Plumber automatically injects several parameters into every Job and Command:
- `url`: The cleaned and parsed URL from the browser.
- `url_hash`: A stable 8-character SHA-256 hash of the URL.

#### Capturing Output
You can capture the `stdout` of a `run` step into a new parameter using the `save_to` field. This parameter can then be used in subsequent steps within the same job.

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
      - run: "<<parameters.browser>> '<<parameters.url>>'"

  save_url_markdown:
    parameters:
      output_dir:
        type: string
        default: "~/Documents/ReadLater"
    steps:
      - run:
          command: "url-hash <<parameters.url>>"
          save_to: "custom_hash"
      - run: "curl -sL '<<parameters.url>>' -o page.html"
      - run: "go-read-md --output <<parameters.output_dir>> --url '<<parameters.url>>' --input page.html --filename '<<parameters.custom_hash>>.md'"
 
  save_html_markdown:
    parameters:
      output_dir:
        type: string
        default: "~/Documents/ReadLater"
    steps:
      - run: "go-read-md --output <<parameters.output_dir>> --url '<<parameters.url>>' --input '{html}' --filename '<<parameters.url_hash>>.md'"


jobs:
  default_firefox:
    steps:
      - open_browser:
          browser: "firefox"

  read_markdown:
    steps:
      - save_url_markdown

  read_html:
    steps:
      - save_html_markdown

workflows:
  smart_routing:
    jobs:
      - read_markdown:
          match: "(?i)(medium\\.com)"
      - read_html:
          match: "(?i)(nytimes\\.com|wsj\\.com)"
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
