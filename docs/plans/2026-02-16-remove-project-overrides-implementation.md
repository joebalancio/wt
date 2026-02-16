# Remove Deprecated project_overrides Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove the deprecated `project_overrides` configuration feature from the codebase.

**Architecture:** Simple code removal - delete deprecated struct/field, update error message, clean up documentation. No new functionality.

**Tech Stack:** Go 1.x, gopkg.in/yaml.v3

---

## Task 1: Remove OverrideConfig Struct and Overrides Field

**Files:**
- Modify: `internal/config/config.go:17-24` (Config struct)
- Modify: `internal/config/config.go:82-90` (OverrideConfig struct)

**Step 1: Remove Overrides field from Config struct**

Edit `internal/config/config.go` line 23, remove:
```go
	Overrides []OverrideConfig `yaml:"project_overrides,omitempty"`
```

The Config struct should end at line 22 (Spice field):
```go
// Config represents the main configuration structure
type Config struct {
	Global    GlobalConfig     `yaml:"global"`
	Hooks     HooksConfig      `yaml:"hooks"`
	Tmux      TmuxConfig       `yaml:"tmux"`
	Worktree  WorktreeConfig   `yaml:"worktree"`
	Spice     SpiceConfig      `yaml:"spice"`
}
```

**Step 2: Remove OverrideConfig struct entirely**

Delete lines 82-90 in `internal/config/config.go`:
```go
// OverrideConfig allows project-specific overrides
//
// Deprecated: Use project-local .wt.yaml files instead.
// This field is kept for backward compatibility but is no longer actively used.
// Project-specific hooks should be defined in a .wt.yaml file at the repository root.
type OverrideConfig struct {
	Match string      `yaml:"match"`
	Hooks HooksConfig `yaml:"hooks,omitempty"`
}
```

**Step 3: Verify build succeeds**

Run: `make build`
Expected: Build succeeds with no errors

**Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "refactor(config): remove deprecated OverrideConfig and Overrides field

The project_overrides feature has been replaced by project-local .wt.yaml files.

Issue: wt-xjh

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 2: Update Error Message in CLI Parser

**Files:**
- Modify: `internal/cli/cli_config_parser.go:192`

**Step 1: Update error message**

Edit `internal/cli/cli_config_parser.go` line 192.

Before:
```go
		return fmt.Errorf("key %q not supported for CLI manipulation\n       Edit config file directly to modify hooks or project_overrides", key)
```

After:
```go
		return fmt.Errorf("key %q not supported for CLI manipulation\n       Edit config file directly to modify hooks", key)
```

**Step 2: Verify build succeeds**

Run: `make build`
Expected: Build succeeds with no errors

**Step 3: Commit**

```bash
git add internal/cli/cli_config_parser.go
git commit -m "refactor(cli): remove project_overrides from error message

Issue: wt-xjh

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 3: Clean Up Documentation

**Files:**
- Modify: `docs/usage.md:202-211` (deprecated section)
- Modify: `docs/usage.md:321` (deprecation notice)

**Step 1: Remove deprecated project_overrides section**

Delete lines 202-211 in `docs/usage.md`:
```yaml
# Project-specific configuration
# DEPRECATED: Use .wt.yaml in the repository root instead
# The project_overrides field is kept for backward compatibility but is no longer used.
# To configure project-specific hooks, create a .wt.yaml file in the repository root.
# project_overrides:
#   - match: "**/*rust*"                 # Matches projects with "rust" in path
#     hooks:
#       on_worktree_create:
#         - run: "cargo fetch"
#           cwd: "{worktree_path}"
```

**Step 2: Remove deprecation notice from Notes section**

Edit `docs/usage.md` around line 318-322.

Before:
```
**Notes:**
- `set` and `unset` commands always modify the global config (`~/.config/wt/config.yaml`)
- For hooks, edit the config file directly (project-local `.wt.yaml` or global config)
- `project_overrides` is deprecated; use project-local `.wt.yaml` files instead
- `get` and `list` respect the config discovery order (flag → local → global)
```

After:
```
**Notes:**
- `set` and `unset` commands always modify the global config (`~/.config/wt/config.yaml`)
- For hooks, edit the config file directly (project-local `.wt.yaml` or global config)
- `get` and `list` respect the config discovery order (flag → local → global)
```

**Step 3: Commit**

```bash
git add docs/usage.md
git commit -m "docs: remove deprecated project_overrides documentation

Issue: wt-xjh

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 4: Final Verification

**Files:**
- None (verification only)

**Step 1: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 2: Run linting**

Run: `make lint`
Expected: No lint warnings

**Step 3: Run full check**

Run: `make check`
Expected: All checks pass

**Step 4: Functional verification**

Run: `./bin/wt config list`
Expected: Config list command works without errors

Run: `./bin/wt config validate`
Expected: Config validation works without errors

**Step 5: Close the bead**

```bash
bd close wt-xjh
```

---

## Summary

| Task | Files Modified | Commits |
|------|----------------|---------|
| 1 | `internal/config/config.go` | 1 |
| 2 | `internal/cli/cli_config_parser.go` | 1 |
| 3 | `docs/usage.md` | 1 |
| 4 | (verification) | 0 |

**Total commits:** 3
