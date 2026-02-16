# Config Local/Global Flags Design

**Issue:** wt-j62
**Date:** 2026-02-16
**Status:** Implemented

## Overview

Add `--local` and `--global` flags to `wt config` commands to enable explicit control over whether commands operate on project-local (`.wt.yaml`) or global (`~/.config/wt/config.yaml`) configuration. This follows git's config command pattern for familiar UX.

## Motivation

Currently:
- `wt config set` / `unset` → writes to **global only**
- `wt config get` / `list` → reads **merged** config

This is asymmetric and unintuitive. Users expect write commands to default to local config (project-specific) with an option for global.

## Proposed Behavior

### Write Commands (set/unset)

| Command | Target | Requires git repo? |
|---------|--------|-------------------|
| `wt config set <key> <val>` | Local (`.wt.yaml`) | Yes |
| `wt config set --global <key> <val>` | Global (`~/.config/wt/config.yaml`) | No |
| `wt config unset <key>` | Local (`.wt.yaml`) | Yes |
| `wt config unset --global <key>` | Global | No |

### Read Commands (get/list)

| Command | Target | Outside git repo? |
|---------|--------|-------------------|
| `wt config get <key>` | Merged (local > global > defaults) | Warning, show global only |
| `wt config get --local <key>` | Local only | Error |
| `wt config get --global <key>` | Global only | Works normally |
| `wt config list` | Merged | Warning, show global only |
| `wt config list --local` | Local only | Error |
| `wt config list --global` | Global only | Works normally |

## Implementation

### New Types and Functions

Add to `internal/cli/cli_config_parser.go`:

```go
// ConfigScope defines which config to target
type ConfigScope int

const (
    ScopeMerged ConfigScope = iota  // Read: merged, Write: local
    ScopeLocal                      // Local only
    ScopeGlobal                     // Global only
)

// Operation defines read vs write context
type Operation int

const (
    OpRead Operation = iota
    OpWrite
)

// ResolveConfigPaths returns the appropriate paths based on scope and operation
func ResolveConfigPaths(scope ConfigScope, op Operation) (projectPath, globalPath string, err error)
```

### Modified Files

| File | Changes |
|------|---------|
| `cli_config_parser.go` | Add `ConfigScope`, `Operation`, `ResolveConfigPaths()` + tests |
| `cli_config_set.go` | Add `--global` flag, use `ResolveConfigPaths()`, update tests |
| `cli_config_unset.go` | Add `--global` flag, use `ResolveConfigPaths()`, update tests |
| `cli_config_get.go` | Add `--local`/`--global` flags, warning for merged outside git, update tests |
| `cli_config_list.go` | Add `--local`/`--global` flags, same behavior as get, update tests |

### Error Handling

**Local write outside git repo:**
```
Error: not in a git repository
Local config requires being in a git repository. Use --global to modify global config.
```

**Local read outside git repo:**
```
Error: not in a git repository
Use --global to read global config.
```

**Merged read outside git repo:**
```
Warning: not in a git repository, showing global config only
tmux.layout: main-horizontal
...
```

**Conflicting flags:**
```
Error: cannot specify both --local and --global
```

## Testing Strategy

1. **Unit tests for `ResolveConfigPaths()`**
   - Test `ScopeGlobal` → returns global path only
   - Test `ScopeLocal` inside git repo → returns project path
   - Test `ScopeLocal` outside git repo → error
   - Test `ScopeMerged` → calls `config.FindConfigs()`

2. **Unit tests for modified commands**
   - Test default behavior (no flag) for each command
   - Test `--local` and `--global` flags
   - Test error scenarios (outside git repo, conflicting flags)
   - Test creation of `.wt.yaml` when it doesn't exist

3. **Integration tests**
   - Use temp directories for test config files
   - Mock `config.FindGitRoot()` for testing "outside git repo" scenarios

## Breaking Changes

**User-facing breaking change:** `wt config set` now defaults to **local** (was global-only).

Migration for users expecting old behavior: Add `--global` flag.

## Follow-up Work

- **wt-jio (P4):** Refactor `internal/cli` for better separation of concerns. Currently `cli_config_parser.go` mixes parsing and path resolution concerns.

## Examples

```bash
# Set project-specific tmux layout
cd ~/my-project
wt config set tmux.layout tiled

# Set global default layout
wt config set --global tmux.layout main-horizontal

# Read effective value (merged)
wt config get tmux.layout
# → shows "tiled" (local wins)

# Read global value explicitly
wt config get --global tmux.layout
# → shows "main-horizontal"

# List all merged config
wt config list

# List only project config
wt config list --local

# Unset project value (reverts to global)
wt config unset tmux.layout
```

## References

- Git config behavior: `git config --help`
- Issue: wt-j62
- Follow-up refactor: wt-jio
