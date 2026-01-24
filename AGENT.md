# AI Agent Development Guide for browser-pipes

This document outlines the principles and workflow for AI agents working on the browser-pipes codebase.

## Core Philosophy: Do One Thing and Do It Right

The browser-pipes project follows Unix philosophy: simple, composable tools that work together. When making changes, focus on clarity, testability, and maintainability.

---

## 5 Golden Rules

### 1. **Configuration First**

All behavior should be driven by configuration, not hardcoded logic.

- **Configuration Schema**: The project uses `plumber.schema.json` (auto-generated from Go structs) to validate YAML configurations
- **Version 2 Format**: All configurations must use `version: 2` with the CircleCI-inspired structure:
  - `commands`: Reusable, parameterized building blocks
  - `jobs`: Compositions of commands/steps
  - `workflows`: Regex-based routing rules that map URLs to jobs

**Before adding features:**
1. Check if it can be expressed in the existing configuration schema
2. If not, extend the schema in `cmd/plumber/config_v2.go`
3. Update `plumber.example.yaml` with working examples
4. Regenerate schema: `make build && ./bin/plumber schema > plumber.schema.json`

**Example**: Instead of hardcoding "send to Firefox", create a parameterized `open_browser` command that accepts a browser name.

---

### 2. **Use Makefile Targets**

All common operations have Makefile targets. Use them.

**Available Targets:**
- `make build` - Build the plumber binary
- `make build-tools` - Build helper tools (go-read-md, etc.)
- `make validate-config` - Validate YAML configuration
- `make test-config` - Test plumber with mock messages
- `make test-read-md` - Test the markdown extraction tool
- `make install-config` - Install default configuration
- `make install-host EXTENSION_ID=...` - Register native messaging host
- `make clean` - Clean build artifacts

**When adding new functionality:**
1. Create a Makefile target for testing it
2. Add it to `.PHONY` declaration
3. Document it in the README.md Makefile Targets table
4. Use existing targets as dependencies (e.g., `test-read-md: build-tools`)

**Example**: When adding `go-read-md`, we created `test-read-md` target with sensible defaults and parameter overrides.

---

### 3. **Keep It Simple**

Complexity is the enemy of reliability.

**Extension (JavaScript):**
- Single context menu item: "Send to Browser Pipe"
- No hardcoded browser/action logic
- Send empty `target` field → let Plumber decide routing
- Minimal UI, maximum flexibility through configuration

**Plumber (Go):**
- Clear separation: `main.go` (CLI), `config_v2.go` (schema), `execution_v2.go` (runtime)
- Strict error handling in native messaging loop
- Structured logging to stderr (never stdout - that's for native messaging protocol)
- URL cleaning happens once, early in the pipeline

**Tools (cmd/):**
- Each tool does ONE thing: `go-read-md` extracts and converts, nothing else
- Accept configuration via flags, not environment variables
- Exit codes: 0 for success, 1 for errors
- Verbose output via `--verbose` flag, not by default

---

### 4. **Test Your Changes**

Every change must be testable via Makefile targets.

**Testing Workflow:**
1. **Validate configuration**: `make validate-config`
2. **Test with mock data**: `make test-config` or `make test-read-md URL=...`
3. **Verify schema**: `./bin/plumber schema` should match `plumber.schema.json`
4. **Manual integration test**: Load extension, test real browser interaction

**When adding features:**
- Add test targets to Makefile
- Use `tools/mocker` to simulate native messaging input
- Test edge cases (empty URLs, invalid configs, network failures)
- Verify error messages are actionable

**Example Test Pattern:**
```makefile
test-my-feature: build
	@echo "🧪 Testing my feature..."
	@./bin/plumber -config test-config.yaml validate
	@echo "✅ Test passed"
```

---

### 5. **Always Update README.md**

The README is the source of truth. Keep it current but concise.

**What to update:**
- **Makefile Targets table**: Add new targets with description and usage
- **Key Features**: Only if adding user-facing functionality
- **Setup & Configuration**: Update if installation steps change
- **Architecture**: Only for major structural changes

**What NOT to do:**
- Don't duplicate information from code comments
- Don't write detailed API documentation (use `go doc` for that)
- Don't include every configuration option (that's what `plumber.example.yaml` is for)

**Efficient Edit Pattern:**
1. Identify the relevant section (Features, Setup, Makefile Targets, etc.)
2. Make surgical edits - add/update 1-3 lines
3. Keep formatting consistent (tables, code blocks, emoji)
4. Test that examples still work

---

## Development Workflow

### Adding a New Tool (e.g., `go-read-md`)

1. **Create the tool**: `cmd/my-tool/main.go`
2. **Add dependencies**: `go get <package>`
3. **Add Makefile target**:
   ```makefile
   build-my-tool:
       go build -o bin/my-tool ./cmd/my-tool
   ```
4. **Create test target**:
   ```makefile
   test-my-tool: build-my-tool
       @./bin/my-tool --test-flag
   ```
5. **Update configuration**: Add command to `plumber.example.yaml`
6. **Validate**: `make validate-config`
7. **Update README**: Add to Makefile Targets table

### Modifying Configuration Schema

1. **Edit Go structs**: `cmd/plumber/config_v2.go`
2. **Add validation logic**: `Config.Validate()` method
3. **Regenerate schema**: `./bin/plumber schema > plumber.schema.json`
4. **Update example**: `plumber.example.yaml`
5. **Test**: `make validate-config`
6. **Document**: Update README if user-facing

### Extending the Extension

1. **Keep it minimal**: Avoid adding UI complexity
2. **Configuration over code**: New behaviors should be config-driven
3. **Test native messaging**: Use browser DevTools console
4. **Verify disconnection handling**: Plumber crashes should be graceful

---

## Common Patterns

### Parameter Substitution in Commands
```yaml
commands:
  my_command:
    parameters:
      param_name:
        type: string
        default: "default_value"
    steps:
      - run: "echo <<parameters.param_name>> {url}"
```

### Regex-Based Routing
```yaml
workflows:
  smart_routing:
    jobs:
      - specific_job:
          match: "(?i)(domain\\.com|other\\.org)"
      - fallback_job:
          match: ".*"
```

### Error Handling in Tools
```go
if err != nil {
    log.Fatalf("❌ Failed to do thing: %v", err)
}
```

---

## Anti-Patterns to Avoid

❌ **Hardcoding browser/action logic in extension**
✅ Use configuration-driven routing in Plumber

❌ **Adding features without Makefile targets**
✅ Every feature gets a test target

❌ **Verbose README updates**
✅ Surgical, efficient edits

❌ **Complex multi-step commands**
✅ Compose simple commands into jobs

❌ **Writing to stdout in Plumber**
✅ Stdout is for native messaging protocol only; use stderr for logs

---

## File Structure Reference

```
browser-pipes/
├── cmd/
│   ├── plumber/          # Main backend (native messaging host)
│   │   ├── main.go       # CLI entry point, subcommands
│   │   ├── config_v2.go  # Configuration schema and validation
│   │   └── execution_v2.go # Workflow execution engine
│   └── go-read-md/       # Article extraction tool
├── extension/
│   ├── background.js     # Extension logic (keep minimal!)
│   └── manifest.json     # Extension metadata
├── tools/
│   └── mocker/           # Native messaging test harness
├── plumber.example.yaml  # Reference configuration
├── plumber.schema.json   # Auto-generated JSON Schema
├── Makefile              # All build/test targets
└── README.md             # User-facing documentation
```

---

## Summary Checklist

Before submitting changes, verify:

- [ ] Configuration changes are reflected in `plumber.example.yaml`
- [ ] Schema is regenerated if config structs changed
- [ ] Makefile target exists for testing the feature
- [ ] `make validate-config` passes
- [ ] README.md is updated (if user-facing)
- [ ] Code follows "do one thing well" philosophy
- [ ] Error messages are actionable and use emoji for clarity (🔧 ❌ ✅ 📝)

---

**Remember**: Simplicity scales. Complexity compounds. When in doubt, choose the simpler path.
