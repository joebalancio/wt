# Remove Unused global.tmux_session_prefix Config Option

**Issue:** wt-blf
**Date:** 2026-02-16
**Status:** Ready for Implementation

## Background

The `global.tmux_session_prefix` config option is defined in the codebase but never consumed. WT's design explicitly does NOT create tmux sessions (only windows), so this option serves no purpose and violates YAGNI.

## Scope

### Files to Modify

| File | Lines | Action |
|------|-------|--------|
| `internal/config/config.go` | 28, 98 | Remove `TmuxSessionPrefix` field from `GlobalConfig` struct and `DefaultConfig()` |
| `internal/cli/cli_config_parser.go` | 118-119, 218-219, 348-349, 423 | Remove from `getGlobalValue()`, `setGlobalValue()`, `unsetGlobalValue()`, and `isSupportedKey()` |
| `internal/cli/cli_config_parser_test.go` | 27, 72, 146-151 | Remove related test cases |
| `internal/cli/cli_config_integration_test.go` | 141, 266-267 | Remove related assertions |
| `AGENTS.md` | 94 | Remove from supported keys list |
| `docs/usage.md` | 233, 261 | Remove from config tables |
| `.wt.yaml.example` | 12 | Remove the field |

### Files to Leave Unchanged

- **`docs/plans/*.md`** - Historical design documents, preserve for reference
- **`docs/tdd-bash-to-go-conversion.md`** - Historical reference document
- **`.beads/issues.jsonl`** - Issue tracker data
- **`.wt.yaml`** - Already updated (does not contain the field)

## Implementation Steps

1. **Remove from config struct** (`internal/config/config.go`)
   - Delete `TmuxSessionPrefix string` field from `GlobalConfig`
   - Delete `TmuxSessionPrefix: "wt-"` from `DefaultConfig()`

2. **Remove from parser** (`internal/cli/cli_config_parser.go`)
   - Remove case `"tmux_session_prefix"` from `getGlobalValue()`
   - Remove case `"tmux_session_prefix"` from `setGlobalValue()`
   - Remove case `"tmux_session_prefix"` from `unsetGlobalValue()`
   - Remove `"global.tmux_session_prefix"` from `isSupportedKey()` map

3. **Remove tests** (`internal/cli/cli_config_parser_test.go`)
   - Remove test case at line 27: `{"global tmux_session_prefix", ...}`
   - Remove test case at line 72: `{"global prefix", ...}`
   - Remove entire test case block at lines 146-151: `unset global tmux_session_prefix`

4. **Remove integration test assertions** (`internal/cli/cli_config_integration_test.go`)
   - Remove line 141: `{"global.tmux_session_prefix", "wt-"}`
   - Remove lines 266-267: The `TmuxSessionPrefix` assertion block

5. **Update documentation** (`AGENTS.md`, `docs/usage.md`)
   - Remove `global.tmux_session_prefix` from supported keys list in AGENTS.md
   - Remove from config tables in docs/usage.md (lines 233 and 261)

6. **Update example config** (`.wt.yaml.example`)
   - Remove `tmux_session_prefix: "wt-"` line

## Verification

After implementation:

```bash
# All tests pass
make test

# Config list no longer shows the option
wt config list | grep -i session_prefix
# (should return nothing)

# Config get returns error for unknown key
wt config get global.tmux_session_prefix
# (should return error: unknown config key)

# Build succeeds
make build
```

## Risk Assessment

**Risk Level:** Low

- No runtime code depends on this field
- No hooks or external integrations reference it
- Removal is purely cleanup, no behavior changes
- Tests will fail immediately if any references are missed

## Notes

- The `global` section of config will still exist (it's empty but valid YAML)
- This is a breaking change for anyone who had this key in their config, but since it was never used, there's no functional impact
- Consider whether to keep an empty `global:` section in `.wt.yaml.example` or remove it entirely if no other global options exist
