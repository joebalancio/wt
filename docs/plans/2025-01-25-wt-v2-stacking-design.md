# WT v2: Stacking Feature Design

**Status:** Design Phase
**Author:** WT Team
**Created:** 2025-01-25

## Overview

WT v2 introduces **branch stacking** capabilities by integrating with [git-spice](https://github.com/abhinav/git-spice), enabling developers to manage stacked feature branches with automatic worktree creation and setup hook execution.

### Goals

1. **Ergonomic CLI** - Flat command structure for common operations
2. **Git-Spice Integration** - Leverage git-spice for stack management
3. **Automatic Setup** - Run hooks after worktree creation
4. **Configurable Paths** - Support both dedicated and per-repo worktree locations
5. **Health Checks** - Validate dependencies and configuration

### Non-Goals

- Native stack metadata tracking (delegate to git-spice)
- Fallback stack detection (git-spice is required)
- PR management (defer to git-spice/gh)

---

## Command Structure

### Design Philosophy

**Primary workflow = flat commands, advanced operations = namespaced**

```
# Primary workflow (flat, no namespace)
wt add <branch>           # Root branch creation
wt list                   # List worktrees (default)
wt remove|rm <path>       # Remove worktree
wt stack [name]           # Stack on current branch
wt stack list             # Show stack hierarchy
wt init                   # First-time setup
wt setup                  # Re-run setup hooks
wt doctor                 # Health check

# Advanced (namespaced, future)
wt config set|list|unset  # Manual config editing
```

### Command Reference

| Command | Args | Flags | Description |
|---------|------|-------|-------------|
| `wt add` | `<branch>` | `--base`, `--path`, `--force`, `--no-setup` | Create root branch + worktree |
| `wt list` | | `--branches`, `--path` | List worktrees (default command) |
| `wt remove` | `<path>` | `--force` | Remove worktree |
| `wt rm` | `<path>` | `--force` | Alias for remove |
| `wt stack` | `[name]` | `--base`, `--force`, `--no-setup` | Create stacked branch |
| `wt stack list` | | | Show stack with paths |
| `wt init` | | | Create config, check deps |
| `wt setup` | | | Re-run setup hooks |
| `wt doctor` | | | Health check |

---

## Stacking Workflow

### Git-Spice Integration

**WT wraps git-spice for all stack operations:**

```bash
# User runs:
wt stack

# WT executes:
gs branch feat/auth-xY7k     # git-spice creates stacked branch
git worktree add <path>       # WT creates worktree
run_setup_hooks()             # WT runs setup
```

### Branch Naming Convention

**Nanoid suffixes (4 characters)** for collision-free unique IDs:

```
feat/auth              (root branch)
feat/auth-xY7k        (auto-suffix)
feat/auth-aB2m        (auto-suffix)
feat/auth-api-k9P2    (named suffix)
```

**Why 4 chars?**
- 64^4 = ~16M combinations
- 99.9% collision-free after ~1000 branches
- Short enough for readable branch names
- URL-safe characters

### Suffix Generation

```go
// If no name provided, generate auto-suffix
if name == "" {
    suffix := nanoid.Generate(4)  // e.g., "xY7k"
    currentBranch := git.GetCurrentBranch()
    name = currentBranch + "-" + suffix
}

// If name provided, append suffix too
if name != "" {
    suffix := nanoid.Generate(4)
    currentBranch := git.GetCurrentBranch()
    name = currentBranch + "-" + name + "-" + suffix
}
```

### Example Workflow

```bash
# 1. Create root branch
wt add feat/auth
# → Creates: feat/auth
# → Path: ~/worktrees/feat/auth
# → Runs: setup hooks
# → Output: Created ~/worktrees/feat/auth

# 2. Enter worktree
cd ~/worktrees/feat/auth

# 3. Stack on current (auto-suffix)
wt stack
# → Calls: gs branch feat/auth-xY7k
# → Creates: feat/auth-xY7k
# → Path: ~/worktrees/feat/auth-xY7k
# → Runs: setup hooks
# → Output: Created ~/worktrees/feat/auth-xY7k

# 4. Enter worktree
cd ~/worktrees/feat/auth-xY7k

# 5. Stack continues
wt stack
# → Calls: gs branch feat/auth-xY7k-aB2m
# → Creates: feat/auth-aB2m
# → Path: ~/worktrees/feat/auth-xY7k-aB2m
# → Runs: setup hooks
# → Output: Created ~/worktrees/feat/auth-xY7k-aB2m

# 6. Enter worktree
cd ~/worktrees/feat/auth-xY7k-aB2m

# 7. Named stack, skip setup
wt stack add-tests --no-setup
# → Calls: gs branch feat/auth-xY7k-aB2m-add-tests-k9P2
# → Creates: feat/auth-add-tests-k9P2
# → Path: ~/worktrees/feat/auth-add-tests-k9P2
# → Skips: setup hooks
# → Output: Created ~/worktrees/feat/auth-add-tests-k9P2
```

### Stack Protection

**Error when stacking on main/master branches:**

```bash
git checkout main
wt stack
# Error: Cannot stack on 'main'. Stack on feature branches only.
# Use --force to override.

wt stack --force
# → Creates: main-xY7k (if you really want this)
```

---

## Stack List Display

### Output Format

**Tree view with paths and current marker:**

```bash
cd ~/worktrees/feat/auth-xY7k
wt stack list
```

**Output:**
```
feat/auth (root)                                     [~/worktrees/feat/auth]
├── feat/auth-xY7k (current) ◀─────────────────────  [~/worktrees/feat/auth-xY7k]
│   ├── feat/auth-xY7k-aB2m                         [~/worktrees/feat/auth-xY7k-aB2m]
│   │   └── feat/auth-xY7k-aB2m-api-fix-k9P2        [~/worktrees/feat/auth-xY7k-aB2m-api-fix]
│   └── feat/auth-xY7m-auth-refactor                [~/worktrees/feat/auth-xY7m-auth-refactor]
└── feat/auth-k9P2                                  [~/worktrees/feat/auth-k9P2]
```

### Current Detection

**Determine current branch by:**
1. Check git HEAD in current directory
2. Match against stack branches
3. Mark with `(current)` and `◀────` indicator

**Implementation:**
```go
func getStackList() StackDisplay {
    // Get stack from git-spice
    branches := callGitSpiceStack()

    // Detect current
    currentBranch := git.GetCurrentBranch()

    // Mark current in display
    for _, b := range branches {
        if b.Name == currentBranch {
            b.IsCurrent = true
        }
    }

    return formatTree(branches)
}
```

---

## Worktree Path Configuration

### Location Options

**Two modes for worktree storage:**

| Mode | Location | Use Case |
|------|----------|----------|
| `dedicated` | `~/worktrees/<branch>` | Centralized, easier cleanup |
| `per-repo` | `<repo>/.worktrees/<branch>` | Project-scoped, portable |

### Config Structure

```yaml
# ~/.config/wt/config.yaml
worktree:
  location: dedicated          # "dedicated" or "per-repo"
  dedicated_path: ~/worktrees  # custom path for dedicated mode
```

### Path Resolution

```go
func getWorktreePath(branch string) string {
    config := loadConfig()

    if config.Worktree.Location == "dedicated" {
        path := config.Worktree.DedicatedPath
        if path == "" {
            path = "~/worktrees"  // default
        }
        return filepath.Join(path, branch)
    }

    // per-repo mode
    repoRoot := git.GetRepoRoot()
    return filepath.Join(repoRoot, ".worktrees", branch)
}
```

---

## Dependency: Git-Spice

### Requirement

**Git-spice is REQUIRED for stacking:**

```bash
wt init
# Checking dependencies...
# ✓ git installed (2.45.1)
# ✗ git-spice not found
#
#   git-spice is required for stacking.
#
#   Install with one of:
#     cargo install git-spice
#     brew install git-spice
#     cargo-binstall git-spice
#
#   Run 'wt init' again after installing.
```

### Detection

```go
func hasGitSpice() bool {
    _, err := exec.LookPath("gs")
    return err == nil
}

func getGitSpiceVersion() (string, error) {
    cmd := exec.Command("gs", "--version")
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(output)), nil
}
```

### Version Check

**Minimum version requirement (optional):**
```go
const MinGitSpiceVersion = "0.7.0"

func checkGitSpiceVersion() error {
    version, err := getGitSpiceVersion()
    if err != nil {
        return fmt.Errorf("git-spice not installed")
    }

    if !versionAtLeast(version, MinGitSpiceVersion) {
        return fmt.Errorf("git-spice %s required, have %s", MinGitSpiceVersion, version)
    }

    return nil
}
```

---

## Setup Hooks

### Automatic Execution

**Hooks run automatically after worktree creation:**

```bash
wt add feat/auth       # Runs setup hooks
wt stack               # Runs setup hooks
wt stack --no-setup    # Skips setup hooks
```

### Hook Definition

```yaml
# ~/.config/wt/config.yaml (global hooks)
hooks:
  post-create:
    - run: "npm install"
      global: true      # Runs for all repos
    - run: "cp .env.example .env"
      global: true

# .wt.yaml (per-repo hooks)
hooks:
  post-create:
    - run: "./scripts/setup.sh"
      global: false     # Runs only for this repo
    - run: "make dev-setup"
      global: false
```

### Manual Re-run

**Re-run setup hooks in current worktree:**

```bash
cd ~/worktrees/feat/auth
wt setup
# → Runs all post-create hooks for current worktree
```

---

## Health Check: wt doctor

### Checks Performed

```bash
wt doctor
```

**Output:**
```
Checking wt installation...
✓ wt binary: /usr/local/bin/wt
✓ Version: v2.0.0

Checking dependencies...
✓ git installed: git version 2.45.1
✓ git worktree supported
✓ git-spice installed: gs version 0.7.0

Checking configuration...
✓ User config: ~/.config/wt/config.yaml
✓ Config is valid YAML
✓ Worktree location: dedicated (~/worktrees)

Checking current repository...
✓ Git repository detected
✓ On feature branch: feat/auth-xY7k
✓ Can create stack (not on main/master)

All checks passed!
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All checks passed |
| 1 | Critical failure (git missing, etc.) |
| 2 | Warning (git-spice missing, config invalid) |

---

## Implementation Phases

### Phase 1: Foundation
- [ ] Add nanoid dependency
- [ ] Implement `wt init` with git-spice detection
- [ ] Implement `wt doctor`
- [ ] Add worktree location config

### Phase 2: Core Stacking
- [ ] Implement `wt stack` (auto-suffix)
- [ ] Implement `wt stack <name>` (named suffix)
- [ ] Add `--base`, `--force`, `--no-setup` flags
- [ ] Integrate with git-spice (`gs branch`)

### Phase 3: Stack Display
- [ ] Implement `wt stack list`
- [ ] Parse git-spice output
- [ ] Add current branch detection
- [ ] Format tree display with paths

### Phase 4: Setup Integration
- [ ] Implement auto-setup in `wt add`
- [ ] Implement auto-setup in `wt stack`
- [ ] Implement `wt setup` command
- [ ] Add `--no-setup` flag

### Phase 5: Polish
- [ ] Add completion scripts
- [ ] Update documentation
- [ ] Integration tests with git-spice
- [ ] Performance benchmarks

---

## Open Questions

1. **Git-spice fallback:** Should `wt stack` work at all without git-spice, or fail fast?
   - **Decision:** Fail fast. `wt init` ensures git-spice is installed.

2. **Branch deletion:** Should `wt remove` also call `gs branch delete`?
   - **Decision:** Not in v2. Users can delete branches manually or via git-spice.

3. **Stack reordering:** Should WT support stack reordering via git-spice?
   - **Decision:** Defer to git-spice CLI (`gs stack reorder`).

4. **PR creation:** Should WT integrate with `gs stack submit` or `gh pr create`?
   - **Decision:** Defer to v3. Users use git-spice/gh directly.

---

## Dependencies

### Go Modules
```
github.com/aidarkhanov/nanoid/v3  # Nanoid generation
```

### External Tools
```
git  >= 2.30.0   # Worktree support
gs   >= 0.7.0    # Git-spice (stacking)
```

---

## References

- [Git-spice Documentation](https://abhinav.github.io/git-spice/)
- [Git Worktree Documentation](https://git-scm.com/docs/git-worktree)
- [Nanoid Specification](https://github.com/ai/nanoid)
- [WT v1 Design](./2025-01-25-go-worktree-manager-design.md)
