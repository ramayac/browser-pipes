# Project Blueprint: Browser Pipe 🚿

**Context:**
The user is a Tech Lead using Kubuntu (Linux) and Windows. They value "Unix-style" tools, objective truth, and local configuration over cloud services. They prefer Go (Golang) for the backend and standard Manifest V3 for the frontend.

## 1. Project Overview
**Browser Pipe** is a local Native Messaging system that treats browser URLs as a data stream. It allows the user to:
1.  **Pipe** a URL from one browser (e.g., Chrome) to another (e.g., Firefox).
2.  **Clean** the URL of tracking parameters (UTM, fbclid) automatically.
3.  **Snapshot** the page content into a clean, readable local file (.md or .html) for archival/reading.
4.  **Log** activity to a local file (for `tail -f` monitoring).

## 2. Architecture

### A. The Plumber (Go Binary)
The core executable running on the host machine.
* **Role:** Listener & Router.
* **Protocol:** Standard Native Messaging (Stdin/Stdout with 4-byte length headers).
* **Dependencies:**
    * `gopkg.in/yaml.v3` (Configuration)
    * `github.com/go-shiori/go-readability` (Content extraction)

### B. The Extension (Manifest V3)
A lightweight extension installed in Chrome, Firefox, Brave, and Edge.
* **Role:** The Sender.
* **Permissions:** `nativeMessaging`, `activeTab`, `contextMenus`.
* **Payload:** Sends a standardized JSON "Envelope" to the Plumber.

---

## 3. Data Structures

### The Envelope (JSON)
Every message sent from the Extension to the Plumber must strictly follow this format:

```json
{
  "id": "uuid-v4-string",
  "origin": "chrome", 
  "url": "https://example.com/article?utm_source=junk",
  "target": "snapshot", 
  "timestamp": 1705531200
}

```

* `origin`: The browser sending the request (configured in the extension or detected).
* `target`: Optional. If empty, the Plumber uses its routing rules. Can be specific ("brave") or an action ("snapshot", "toggle").

---

## 4. Configuration (`~/.config/browser-pipe/plumber.yaml`)

The Go binary must be completely configurable via YAML.

```yaml
settings:
  # Folder to save snapshots
  snapshot_folder: "~/ReadingRoom"
  # Formats to save: html (clean DOM), md (markdown)
  snapshot_formats: ["html", "md"] 

browsers:
  # Aliases mapped to system commands
  chrome: "google-chrome"
  firefox: "firefox"
  brave: "brave-browser"
  edge: "microsoft-edge-stable"

toggles:
  # Logic: If origin is X, default target is Y
  chrome: "firefox"
  firefox: "chrome"
  brave: "chrome"

rules:
  # Regex-based routing
  - match: ".*telus.*|.*jira.*"
    target: "chrome"
  - match: "medium.com|substack.com"
    target: "snapshot" # Special action

```

---

## 5. Functional Requirements

### Feature 1: The Toggle

* **Logic:** If the user clicks the extension icon, the `target` in the Envelope is "toggle".
* **Backend:** The Plumber looks up the `origin` in the `toggles` map (from YAML) and executes the corresponding browser command with the URL.

### Feature 2: The Cleaner

* **Logic:** Before any routing or snapshotting, the Plumber must parse the URL and strip known tracking parameters:
* `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`
* `fbclid`, `gclid`, `ref`



### Feature 3: The Snapshot

* **Logic:** If `target` is "snapshot" (or a regex rule matches).
* **Action:**
1. Clean the URL.
2. Fetch the page (HTTP Get).
3. Pass through `go-readability` to extract the main content.
4. Save file to `snapshot_folder` using format: `YYYY-MM-DD-HHMM-[Title-Slug].[ext]`.
5. **Crucial:** Automatically open the resulting local file in the `default_target` browser.



### Feature 4: Logging (Unix Style)

* **Logic:** The Plumber must write structured logs to `Stderr`.
* **Format:** `[TIMESTAMP] [ORIGIN] -> [TARGET] : [URL]`
* **Goal:** User can run `tail -f` on the log file configured in the Native Messaging manifest.
