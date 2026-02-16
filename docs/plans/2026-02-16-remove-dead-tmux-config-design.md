# Remove Dead Tmux Config from wt Configuration System

**Date:** 2026-02-16
**Issue:** wt-9i2
**Priority:** P1

## Problem

The tmux configuration in wt defines several settings that are never consumed by any code:

- `tmux.layout` - CreateSession() exists but is never called
- `tmux.window_name` - Same, session-level setting never used
- `tmux.attach_on_create` - Same
- `tmux.window_naming.max_length` - Hardcoded to 16 in GenerateWindowName()
- `tmux.window_naming.abbreviate_issue_id` - Never read by any code
- `global.tmux_session_prefix` - CreateSession() never called

The config was designed for a session-based workflow (creating new tmux sessions per worktree) but the actual implementation uses a window-based workflow (creating windows in the current session via CreateOrSelectWindow).

## Solution

Remove all dead tmux configuration structs, functions, and documentation while preserving the active window-based tmux operations.

## Design

### Section 1: Config Struct Changes

**File:** `internal/config/config.go`

Remove these structs entirely:
- `GlobalConfig` struct (now empty after removing TmuxSessionPrefix)
- `TmuxConfig` struct
- `TmuxWindowNamingConfig` struct

Remove fields from Config struct:
- `Global GlobalConfig` field
- `Tmux TmuxConfig` field

Update `DefaultConfig()`:
- Remove Global block
- Remove Tmux block

**Result:**
```go
type Config struct {
    Hooks     HooksConfig      `yaml:"hooks"`
    Worktree  WorktreeConfig   `yaml:"worktree"`
    Spice     SpiceConfig      `yaml:"spice"`
    Overrides []OverrideConfig `yaml:"project_overrides,omitempty"`
}
```

### Section 2: CLI Config Parser Changes

**File:** `internal/cli/cli_config_parser.go`

Remove these functions entirely:
- `getGlobalValue()` - no more global section
- `setGlobalValue()` - no more global section
- `unsetGlobalValue()` - no more global section
- `getTmuxValue()` - no more tmux section
- `getTmuxWindowNamingValue()`
- `setTmuxValue()`
- `setTmuxWindowNamingValue()`
- `unsetTmuxValue()`
- `unsetTmuxWindowNamingValue()`

Update `GetValue()`:
- Remove "global" case
- Remove "tmux" case
- Keep only: "worktree" case

Update `SetValue()`:
- Remove "global" case
- Remove "tmux" case
- Keep only: "worktree" case

Update `UnsetValue()`:
- Remove "global" case
- Remove "tmux" case
- Keep only: "worktree" case

Update `isSupportedKey()`:
```go
supportedKeys := map[string]bool{
    "worktree.location":       true,
    "worktree.dedicated_path": true,
}
```

### Section 3: Tmux Client Changes

**File:** `internal/tmux/session.go`

Remove these entirely (all dead code):
- `Session` struct
- `ListSessions()` method
- `HasSession()` method
- `CreateSession()` method
- `AttachSession()` method
- `KillSession()` method
- `parseSessionList()` helper function

**Keep:** All window operations, naming functions, and helpers:
- `CreateNewWindow()`, `SelectWindow()`, `KillWindow()`, `WindowExists()`, `SendKeys()`
- `CreateOrSelectWindow()` - main entry point for worktree workflow
- `GenerateWindowName()`, `GenerateStackWindowName()` - smart naming functions
- All helper functions (extractIssueID, hashBranch, truncate, abbreviatePrefix, etc.)
- `IsInTmux()` - environment check

### Section 4: Example Config & Documentation

**File:** `.wt.yaml.example`

Remove:
- `global:` section with `tmux_session_prefix`
- `tmux:` section with all nested config
- tmux references in project-local example section

**File:** `AGENTS.md`

Remove:
- `wt config unset tmux.attach_on_create` example
- All tmux-related supported keys from config command section:
  - `global.tmux_session_prefix`
  - `tmux.layout`
  - `tmux.window_name`
  - `tmux.attach_on_create`
  - `tmux.window_naming.max_length`
  - `tmux.window_naming.abbreviate_issue_id`

**File:** `docs/usage.md`

Remove:
- Config table entries for tmux keys
- Example commands using tmux keys

### Section 5: Test Changes

**1. `internal/cli/cli_config_parser_test.go`** (most extensive)

Remove test cases for:
- `TestGetValue` - all `tmux.*` cases
- `TestSetValue` - boolean validation with `tmux.attach_on_create`
- `TestSetValue` - `tmux.layout`, `tmux.window_name`
- `TestUnsetValue` - `tmux.*` cases
- `TestDefaultValue` - `TmuxSessionPrefix` assertion
- `TestSetValueWindowNaming` - entire test
- `TestUnsetWindowNaming` - entire test

**2. `internal/cli/cli_config_get_test.go`**
- Remove `tmux.layout` test case

**3. `internal/cli/cli_config_set_test.go`**
- Remove `tmux.layout` test cases

**4. `internal/cli/cli_config_unset_test.go`**
- Remove `tmux.layout` test cases

**5. `internal/cli/cli_config_integration_test.go`**
- Remove tmux entries from default values table
- Remove `SetValue` tests for `tmux.attach_on_create`
- Remove `TmuxSessionPrefix` assertion

## Implementation Order

**Phase 1: Core changes (make code compile)**
1. `internal/config/config.go` - Remove GlobalConfig, TmuxConfig, TmuxWindowNamingConfig structs and defaults
2. `internal/cli/cli_config_parser.go` - Remove global/tmux functions, update switch statements, update ValidKeys map
3. `internal/tmux/session.go` - Remove Session struct and all session methods

**Phase 2: Documentation**
4. `.wt.yaml.example` - Remove global and tmux sections
5. `AGENTS.md` - Remove tmux config references
6. `docs/usage.md` - Remove tmux config references

**Phase 3: Tests**
7. Update 5 test files to remove tmux-related test cases

## Verification Commands

```bash
# Should error with unknown key
wt config get tmux.layout
wt config get tmux.window_name
wt config get global.tmux_session_prefix

# Should not show any tmux.* keys
wt config list

# All tests should pass
make test

# Build should succeed
make build
```

## Success Criteria

- `wt config list` no longer shows any tmux.* keys or global.* keys
- `wt config get tmux.layout` returns error for unknown key
- All tests pass
- Binary builds successfully
- No breaking changes for users (dead config was never functional)

## Backward Compatibility

Users with existing tmux config in their `.wt.yaml` will receive unknown key errors when running `wt config get tmux.*`. This is acceptable because:
1. These settings were never functional
2. No behavior changes for actual tmux window creation
3. The `--no-tmux` flag continues to work

## Files Modified

| File | Change |
|------|--------|
| `internal/config/config.go` | Remove GlobalConfig, TmuxConfig structs |
| `internal/cli/cli_config_parser.go` | Remove global/tmux key handlers |
| `internal/tmux/session.go` | Remove session methods |
| `.wt.yaml.example` | Remove global/tmux sections |
| `AGENTS.md` | Remove tmux config docs |
| `docs/usage.md` | Remove tmux config docs |
| `internal/cli/cli_config_parser_test.go` | Remove tmux test cases |
| `internal/cli/cli_config_get_test.go` | Remove tmux test case |
| `internal/cli/cli_config_set_test.go` | Remove tmux test cases |
| `internal/cli/cli_config_unset_test.go` | Remove tmux test cases |
| `internal/cli/cli_config_integration_test.go` | Remove tmux assertions |

## Files NOT Modified

Historical design documents in `docs/plans/` are left unchanged as historical records.
