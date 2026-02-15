# Project-Local Configuration Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable project-local `.wt.yaml` configuration that layers on top of global config, with Git root discovery and array replacement semantics. This replaces the previous `project_overrides` design.

**Architecture:** Config discovery traverses to Git root using `git rev-parse --show-toplevel` to find `.wt.yaml`. Global config is loaded first, then project config is overlaid using YAML unmarshaling (last-write-wins). Arrays replace entirely, undefined fields inherit from global.

**Tech Stack:** Go 1.21+, `yaml.v3` for overlay merging, existing git client for root discovery

---

## Precedence Hierarchy

```
Highest Priority (most specific):
  ↓ .wt.yaml at Git root (project-local, version controlled)
  ↓ ~/.config/wt/config.yaml (user-global)
Lowest Priority (least specific):
```

**Merge Semantics:**
- **Scalars (strings, bools, numbers)**: Project value replaces global value
- **Arrays (hooks)**: Project array replaces global array entirely
- **Undefined fields**: Keep global value (inherit)

---

## Configuration Schema

Project config (`.wt.yaml` at Git root) uses the same schema as global config:

```yaml
# .wt.yaml (committed to repo for team sharing)
hooks:
  on_worktree_create:
    - run: "cargo fetch"
      cwd: "{worktree_path}"
  on_worktree_remove:
    - run: "rm -rf target"
      cwd: "{worktree_path}"

tmux:
  layout: "main-vertical"
  attach_on_create: false

worktree:
  location: "per-repo"
```

Global config (`~/.config/wt/config.yaml`) provides defaults:

```yaml
# Global defaults
hooks:
  on_worktree_create:
    - run: "npm install"
      cwd: "{worktree_path}"

tmux:
  layout: "main-vertical"
  attach_on_create: true
```

**Result when both exist:**
```yaml
hooks:
  on_worktree_create:
    - run: "cargo fetch"        # Project replaces global
tmux:
  layout: "main-vertical"        # Inherited (undefined in project)
  attach_on_create: false        # Project overrides global
worktree:
  location: "per-repo"           # From project
```

---

## Config Discovery

### Current Behavior
```go
// FindConfig checks: --config flag → .wt.yaml (cwd) → ~/.config/wt/config.yaml
// Returns: single config path
```

### New Behavior
```go
// FindConfigs returns project and global config paths
// projectPath: .wt.yaml at Git root (may be "")
// globalPath: ~/.config/wt/config.yaml (may be "")
func FindConfigs(customPath string) (projectPath, globalPath string, err error)
```

**Discovery Algorithm:**
```
1. If --config flag provided → use explicit path, skip merging
2. Run git rev-parse --show-toplevel to find Git root
   - If not in Git repo → projectPath = ""
   - If Git root found → check for .wt.yaml at root
3. Check ~/.config/wt/config.yaml for global config
4. Return (projectPath, globalPath)
```

**Worktree Behavior:**
- When running `wt` from a worktree, `git rev-parse --show-toplevel` returns the worktree directory
- `.wt.yaml` is committed to the repo, so every worktree has it at its root
- Each worktree gets the same project config (team sharing works automatically)

**Example:**
```
Directory: /home/user/projects/myrepo/.worktrees/mobile/src/
git rev-parse --show-toplevel → /home/user/projects/myrepo/.worktrees/mobile/
Config found: /home/user/projects/myrepo/.worktrees/mobile/.wt.yaml
```

---

## Config Merging

### Implementation (YAML Overlay)

```go
func LoadMerged(projectPath, globalPath string) (*Config, error) {
    cfg := DefaultConfig()

    // Load global config (if exists)
    if globalPath != "" {
        data, err := os.ReadFile(globalPath)
        if err != nil {
            return nil, fmt.Errorf("reading global config: %w", err)
        }
        if err := yaml.Unmarshal(data, cfg); err != nil {
            return nil, fmt.Errorf("parsing global config: %w", err)
        }
    }

    // Overlay project config (if exists)
    if projectPath != "" {
        data, err := os.ReadFile(projectPath)
        if err != nil {
            return nil, fmt.Errorf("reading project config: %w", err)
        }
        if err := yaml.Unmarshal(data, cfg); err != nil {
            return nil, fmt.Errorf("parsing project config: %w", err)
        }
    }

    return cfg, nil
}
```

### Merge Semantics (via yaml.v3)

YAML unmarshaling to the same struct naturally implements "last write wins":
- Fields defined in project YAML overwrite global values
- Fields undefined in project YAML retain global values
- Arrays are replaced entirely (not merged)

---

## Data Flow

```
User runs: wt add feature-branch
    ↓
FindConfigs() discovers:
    - projectPath: /repo/.wt.yaml (from git rev-parse --show-toplevel)
    - globalPath: ~/.config/wt/config.yaml
    ↓
LoadMerged() builds config:
    1. Start with defaults
    2. Apply global config
    3. Overlay project config
    ↓
runSetupHooks() runs cfg.Hooks.OnWorktreeCreate
    (project hooks only, if defined; global hooks only, if no project)
```

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Neither config exists | Return error: "no configuration file found" |
| Project config only | Use project config (no error) |
| Global config only | Use global config (no error) |
| Invalid YAML syntax | Fail with file path and error details |
| Not in Git repo | Skip project config, use global (with warning) |
| Git not available | Skip project config, use global |

---

## Testing Strategy

### Testing Pyramid

```
        ▲
       ╱ ╲
      ╱   ╲     E2E/Manual (1-2 tests)
     ╱─────╲    - Full workflow with wt binary
    ╱       ╲
   ╱─────────╲  Integration Tests (3-4 tests)
  ╱           ╲ - Real Git repos, worktrees
 ╱─────────────╲
╱               ╲ Unit Tests (8-10 tests)
─────────────────  - Config discovery, merging
```

### Unit Tests

| Test | Description |
|------|-------------|
| `TestFindConfigs_ProjectConfig` | Finds `.wt.yaml` at Git root |
| `TestFindConfigs_NoProjectConfig` | Returns empty project path when no `.wt.yaml` |
| `TestFindConfigs_GlobalOnly` | Not in Git repo → projectPath = "" |
| `TestFindConfigs_ExplicitPath` | --config flag bypasses discovery |
| `TestLoadMerged_ScalarsOverride` | Project scalar replaces global |
| `TestLoadMerged_ArraysReplace` | Project array replaces global entirely |
| `TestLoadMerged_UndefinedInherits` | Undefined project field inherits global |
| `TestLoadMerged_ProjectOnly` | No global → project works alone |
| `TestLoadMerged_GlobalOnly` | No project → global works alone |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestIntegration_ProjectConfig_TeamSharing` | Worktree has `.wt.yaml` at root, hooks run |
| `TestIntegration_GlobalFallback` | No project config → global hooks run |
| `TestIntegration_OverrideBehavior` | Both configs → project arrays replace |
| `TestIntegration_GitRootDiscovery` | Run from subdir, finds config at root |

---

## Migration Notes

**Removing `project_overrides`:**
- The `project_overrides` key in global config is no longer needed
- Users should move override patterns to project-local `.wt.yaml` files
- No breaking change: `project_overrides` key is simply ignored if present

**Backward Compatibility:**
- Existing global configs work unchanged
- Adding a `.wt.yaml` to a project is additive (new feature)
- `--config` flag behavior unchanged

---

## Implementation Files

| File | Changes |
|------|---------|
| `internal/config/config.go` | Add `FindConfigs()`, `LoadMerged()`, update callers |
| `internal/config/config_test.go` | Unit tests for discovery and merging |
| `internal/cli/add.go` | Use new config loading |
| `internal/worktree/service.go` | Use new config loading |
| `tests/project_config_integration_test.go` | Integration tests |

---

## Summary

| Aspect | Decision |
|--------|----------|
| **Config location** | `.wt.yaml` at Git root (via `git rev-parse --show-toplevel`) |
| **Precedence** | Project → Global → Defaults |
| **Merge behavior** | Scalars replace, arrays replace, undefined inherits |
| **Team sharing** | `.wt.yaml` committed to repo, worktrees get same config |
| **Complexity** | Low - leverages YAML library behavior |
