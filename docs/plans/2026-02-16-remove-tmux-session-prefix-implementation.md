# Remove Unused global.tmux_session_prefix Config Option - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove the unused `global.tmux_session_prefix` config option from the wt codebase.

**Architecture:** This is a pure cleanup task - removing dead code. The config field exists but is never consumed. WT creates tmux windows, not sessions, so this prefix was never used. We'll remove it from the config struct, CLI parser, tests, and documentation.

**Tech Stack:** Go 1.21+, gopkg.in/yaml.v3, golangci-lint

---

## Task 1: Remove from Config Struct

**Files:**
- Modify: `internal/config/config.go:26-29, 94-99`

**Step 1: Remove TmuxSessionPrefix field from GlobalConfig struct**

In `internal/config/config.go`, find lines 26-29:

```go
// GlobalConfig contains global settings
type GlobalConfig struct {
	TmuxSessionPrefix string `yaml:"tmux_session_prefix"`
}
```

Replace with:

```go
// GlobalConfig contains global settings
type GlobalConfig struct {
}
```

**Step 2: Remove default value from DefaultConfig()**

In `internal/config/config.go`, find lines 94-99:

```go
// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Global: GlobalConfig{
			TmuxSessionPrefix: "wt-",
		},
```

Replace with:

```go
// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Global: GlobalConfig{},
```

**Step 3: Run tests to verify change**

Run: `cd /home/claude/projects/wt && go test ./internal/config/... -v`
Expected: PASS (config package has no tests that reference TmuxSessionPrefix)

**Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "refactor(config): remove unused TmuxSessionPrefix from GlobalConfig"
```

---

## Task 2: Remove from CLI Parser - GetValue

**Files:**
- Modify: `internal/cli/cli_config_parser.go:115-123`

**Step 1: Remove tmux_session_prefix case from getGlobalValue()**

In `internal/cli/cli_config_parser.go`, find lines 115-123:

```go
// getGlobalValue retrieves a global config value
func getGlobalValue(cfg *config.Config, field string) (interface{}, error) {
	switch field {
	case "tmux_session_prefix":
		return cfg.Global.TmuxSessionPrefix, nil
	default:
		return nil, fmt.Errorf("unknown key: global.%s", field)
	}
}
```

Replace with:

```go
// getGlobalValue retrieves a global config value
func getGlobalValue(cfg *config.Config, field string) (interface{}, error) {
	switch field {
	default:
		return nil, fmt.Errorf("unknown key: global.%s", field)
	}
}
```

**Step 2: Run tests to verify change**

Run: `cd /home/claude/projects/wt && go test ./internal/cli/... -run TestGetValue -v`
Expected: FAIL - test at line 27 references `global.tmux_session_prefix`

**Step 3: Commit**

```bash
git add internal/cli/cli_config_parser.go
git commit -m "refactor(cli): remove tmux_session_prefix from getGlobalValue"
```

---

## Task 3: Remove from CLI Parser - SetValue

**Files:**
- Modify: `internal/cli/cli_config_parser.go:215-224`

**Step 1: Remove tmux_session_prefix case from setGlobalValue()**

In `internal/cli/cli_config_parser.go`, find lines 215-224:

```go
// setGlobalValue sets a global config value
func setGlobalValue(cfg *config.Config, field, value string) error {
	switch field {
	case "tmux_session_prefix":
		cfg.Global.TmuxSessionPrefix = value
		return nil
	default:
		return fmt.Errorf("unknown key: global.%s", field)
	}
}
```

Replace with:

```go
// setGlobalValue sets a global config value
func setGlobalValue(cfg *config.Config, field, value string) error {
	switch field {
	default:
		return fmt.Errorf("unknown key: global.%s", field)
	}
}
```

**Step 2: Run tests to verify change**

Run: `cd /home/claude/projects/wt && go test ./internal/cli/... -run TestSetValue -v`
Expected: FAIL - test at line 72 references `global.tmux_session_prefix`

**Step 3: Commit**

```bash
git add internal/cli/cli_config_parser.go
git commit -m "refactor(cli): remove tmux_session_prefix from setGlobalValue"
```

---

## Task 4: Remove from CLI Parser - UnsetValue

**Files:**
- Modify: `internal/cli/cli_config_parser.go:345-354`

**Step 1: Remove tmux_session_prefix case from unsetGlobalValue()**

In `internal/cli/cli_config_parser.go`, find lines 345-354:

```go
// unsetGlobalValue unsets a global config value to default
func unsetGlobalValue(cfg *config.Config, field string) error {
	switch field {
	case "tmux_session_prefix":
		cfg.Global.TmuxSessionPrefix = "wt-" // default
		return nil
	default:
		return fmt.Errorf("unknown key: global.%s", field)
	}
}
```

Replace with:

```go
// unsetGlobalValue unsets a global config value to default
func unsetGlobalValue(cfg *config.Config, field string) error {
	switch field {
	default:
		return fmt.Errorf("unknown key: global.%s", field)
	}
}
```

**Step 2: Run tests to verify change**

Run: `cd /home/claude/projects/wt && go test ./internal/cli/... -run TestUnsetValue -v`
Expected: FAIL - test at lines 146-154 references `global.tmux_session_prefix`

**Step 3: Commit**

```bash
git add internal/cli/cli_config_parser.go
git commit -m "refactor(cli): remove tmux_session_prefix from unsetGlobalValue"
```

---

## Task 5: Remove from CLI Parser - isSupportedKey

**Files:**
- Modify: `internal/cli/cli_config_parser.go:420-433`

**Step 1: Remove global.tmux_session_prefix from supportedKeys map**

In `internal/cli/cli_config_parser.go`, find lines 420-433:

```go
// isSupportedKey returns true if key can be manipulated via CLI
func isSupportedKey(key string) bool {
	supportedKeys := map[string]bool{
		"global.tmux_session_prefix":             true,
		"worktree.location":                      true,
		"worktree.dedicated_path":                true,
		"tmux.layout":                            true,
		"tmux.window_name":                       true,
		"tmux.attach_on_create":                  true,
		"tmux.window_naming.max_length":          true,
		"tmux.window_naming.abbreviate_issue_id": true,
	}
	return supportedKeys[key]
}
```

Replace with:

```go
// isSupportedKey returns true if key can be manipulated via CLI
func isSupportedKey(key string) bool {
	supportedKeys := map[string]bool{
		"worktree.location":                      true,
		"worktree.dedicated_path":                true,
		"tmux.layout":                            true,
		"tmux.window_name":                       true,
		"tmux.attach_on_create":                  true,
		"tmux.window_naming.max_length":          true,
		"tmux.window_naming.abbreviate_issue_id": true,
	}
	return supportedKeys[key]
}
```

**Step 2: Run tests to verify change**

Run: `cd /home/claude/projects/wt && go test ./internal/cli/... -run TestGetValue -v`
Expected: FAIL - test references unsupported key

**Step 3: Commit**

```bash
git add internal/cli/cli_config_parser.go
git commit -m "refactor(cli): remove global.tmux_session_prefix from supported keys"
```

---

## Task 6: Remove Unit Test Cases

**Files:**
- Modify: `internal/cli/cli_config_parser_test.go:27, 72, 145-154`

**Step 1: Remove test case from TestGetValue (line 27)**

In `internal/cli/cli_config_parser_test.go`, find the test table in `TestGetValue` starting around line 16.

Remove line 27:

```go
		{"global tmux_session_prefix", "global.tmux_session_prefix", "wt-", false},
```

**Step 2: Remove test case from TestSetValue (line 72)**

In `internal/cli/cli_config_parser_test.go`, find the test table in `TestSetValue` starting around line 52.

Remove line 72:

```go
		{"global prefix", "global.tmux_session_prefix", "test-", false},
```

**Step 3: Remove test case from TestUnsetValue (lines 145-154)**

In `internal/cli/cli_config_parser_test.go`, find the test table in `TestUnsetValue` starting around line 88.

Remove the entire test case block at lines 145-154:

```go
		{
			name:      "unset global tmux_session_prefix",
			key:       "global.tmux_session_prefix",
			wantError: false,
			validate: func(cfg *config.Config) {
				if cfg.Global.TmuxSessionPrefix != "wt-" {
					t.Errorf("expected default 'wt-', got %q", cfg.Global.TmuxSessionPrefix)
				}
			},
		},
```

**Step 4: Run tests to verify change**

Run: `cd /home/claude/projects/wt && go test ./internal/cli/... -run "TestGetValue|TestSetValue|TestUnsetValue" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/cli_config_parser_test.go
git commit -m "test(cli): remove tmux_session_prefix test cases"
```

---

## Task 7: Remove Integration Test Assertions

**Files:**
- Modify: `internal/cli/cli_config_integration_test.go:141, 265-268`

**Step 1: Remove test case from TestGetCommandIntegration (line 141)**

In `internal/cli/cli_config_integration_test.go`, find the test table in `TestGetCommandIntegration` starting around line 133.

Remove line 141:

```go
		{"global.tmux_session_prefix", "wt-"},
```

**Step 2: Remove assertion from TestListOutput (lines 265-268)**

In `internal/cli/cli_config_integration_test.go`, find `TestListOutput` function starting around line 253.

Remove lines 265-268:

```go
	if cfg.Global.TmuxSessionPrefix == "" {
		t.Error("expected Global.TmuxSessionPrefix to be set")
	}
```

**Step 3: Run tests to verify change**

Run: `cd /home/claude/projects/wt && go test ./internal/cli/... -run "TestGetCommandIntegration|TestListOutput" -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/cli/cli_config_integration_test.go
git commit -m "test(cli): remove tmux_session_prefix integration test assertions"
```

---

## Task 8: Update AGENTS.md Documentation

**Files:**
- Modify: `AGENTS.md:94`

**Step 1: Remove global.tmux_session_prefix from supported keys list**

In `AGENTS.md`, find the "Supported keys:" list starting around line 93.

Remove line 94:

```markdown
- `global.tmux_session_prefix` - Tmux session prefix
```

The resulting list should start with:

```markdown
**Supported keys:**
- `worktree.location` - Worktree location mode (dedicated/per-repo)
```

**Step 2: Verify documentation is correct**

Run: `grep -n "tmux_session_prefix" AGENTS.md`
Expected: No output (no remaining references)

**Step 3: Commit**

```bash
git add AGENTS.md
git commit -m "docs: remove global.tmux_session_prefix from AGENTS.md"
```

---

## Task 9: Update docs/usage.md

**Files:**
- Modify: `docs/usage.md:233, 261`

**Step 1: Remove from NOT IMPLEMENTED section (line 233)**

In `docs/usage.md`, find the "NOT IMPLEMENTED - Future Features:" section around line 230.

Remove line 233:

```markdown
- `global.tmux_session_prefix` - Tmux integration is planned for a future release
```

**Step 2: Remove from Supported Keys table (line 261)**

In `docs/usage.md`, find the "Supported Keys:" table around line 257.

Remove line 261:

```markdown
| `global.tmux_session_prefix` | string | `wt-` | Prefix for tmux session names |
```

**Step 3: Verify documentation is correct**

Run: `grep -n "tmux_session_prefix" docs/usage.md`
Expected: No output (no remaining references)

**Step 4: Commit**

```bash
git add docs/usage.md
git commit -m "docs: remove global.tmux_session_prefix from usage.md"
```

---

## Task 10: Update .wt.yaml.example

**Files:**
- Modify: `.wt.yaml.example:10-12`

**Step 1: Remove global section with tmux_session_prefix**

In `.wt.yaml.example`, find lines 10-12:

```yaml
global:
  # Prefix for tmux session names
  tmux_session_prefix: "wt-"
```

Remove these 3 lines entirely (lines 10-12).

The file should go directly from the header comments to the `hooks:` section.

**Step 2: Verify example file is valid YAML**

Run: `python3 -c "import yaml; yaml.safe_load(open('.wt.yaml.example'))"`
Expected: No error output

**Step 3: Commit**

```bash
git add .wt.yaml.example
git commit -m "docs: remove global.tmux_session_prefix from example config"
```

---

## Task 11: Run Full Test Suite and Verify

**Files:**
- None (verification only)

**Step 1: Run all tests**

Run: `cd /home/claude/projects/wt && make test`
Expected: PASS (all tests pass)

**Step 2: Run linter**

Run: `cd /home/claude/projects/wt && make lint`
Expected: No errors

**Step 3: Build the binary**

Run: `cd /home/claude/projects/wt && make build`
Expected: Binary created at `bin/wt`

**Step 4: Verify config get returns error for removed key**

Run: `./bin/wt config get global.tmux_session_prefix`
Expected: Error message containing "not supported" or "unknown"

**Step 5: Verify config list doesn't show removed key**

Run: `./bin/wt config list | grep -i session_prefix`
Expected: No output (key no longer exists)

**Step 6: Final commit (if any remaining changes)**

```bash
git status
# If clean, no commit needed
```

---

## Summary

| Task | Description | Files Changed |
|------|-------------|---------------|
| 1 | Remove from config struct | `internal/config/config.go` |
| 2 | Remove from GetValue | `internal/cli/cli_config_parser.go` |
| 3 | Remove from SetValue | `internal/cli/cli_config_parser.go` |
| 4 | Remove from UnsetValue | `internal/cli/cli_config_parser.go` |
| 5 | Remove from isSupportedKey | `internal/cli/cli_config_parser.go` |
| 6 | Remove unit test cases | `internal/cli/cli_config_parser_test.go` |
| 7 | Remove integration test assertions | `internal/cli/cli_config_integration_test.go` |
| 8 | Update AGENTS.md | `AGENTS.md` |
| 9 | Update docs/usage.md | `docs/usage.md` |
| 10 | Update example config | `.wt.yaml.example` |
| 11 | Verify all tests pass | - |

**Total: 7 files modified, ~30 lines removed**
