# wt Usage Guide

This guide provides detailed usage examples for all wt commands.

## Table of Contents

- [Global Options](#global-options)
- [Worktree Commands](#worktree-commands)
  - [wt worktree add](#wt-worktree-add)
  - [wt worktree list](#wt-worktree-list)
  - [wt worktree remove](#wt-worktree-remove)
- [Configuration](#configuration)
- [Common Workflows](#common-workflows)

---

## Global Options

These options can be used with any wt command:

| Option | Description |
|--------|-------------|
| `-c, --config <path>` | Specify custom config file path |
| `-v, --verbose` | Increase verbosity (can be used multiple times) |
| `--dry-run` | Show what would be done without executing |

### Examples

```bash
# Use a custom config file
wt -c /path/to/custom.yaml worktree list

# Run in verbose mode
wt -v worktree add feature-branch

# See what would happen without making changes
wt --dry-run worktree remove /path/to/worktree
```

---

## Worktree Commands

### wt worktree add

Add a new worktree for the specified branch.

**Synopsis:**

```bash
wt worktree add <branch> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--base <branch>` | Base branch for new branch (default: current HEAD) |
| `--path <path>` | Custom path for the worktree |
| `--force` | Force creation even if path exists |
| `--no-checkout` | Don't checkout the branch |

**Examples:**

```bash
# Create a new branch from current HEAD and add worktree
wt worktree add feature/login

# Create a new branch from main and add worktree
wt worktree add feature/auth --base main

# Create a worktree at a custom path
wt worktree add hotfix-123 --path ~/projects/hotfix

# Create worktree without checking out files
wt worktree add experiment --no-checkout
```

**How it works:**

1. The command ALWAYS creates a new branch with the specified name
2. The new branch is created from the base branch (default: current HEAD)
3. The worktree is created at `<worktree_root>/<branch>` by default, or at a custom path if specified
4. If a branch with the same name already exists, the command will fail

---

### wt worktree list

List all git worktrees in the current repository.

**Synopsis:**

```bash
wt worktree list [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--branches <names>` | Filter by branch names (comma-separated) |
| `--path <prefix>` | Filter by path prefix |

**Examples:**

```bash
# List all worktrees
wt worktree list

# Filter by specific branches
wt worktree list --branches feature/login,feature/auth

# Filter by path prefix
wt worktree list --path ~/projects/

# Running wt without arguments is equivalent to list
wt
```

**Output format:**

```
/path/to/main-repo        main
/path/to/worktree-1       feature/login
/path/to/worktree-2       feature/auth
```

---

### wt worktree remove

Remove a worktree from the repository.

**Synopsis:**

```bash
wt worktree remove <path> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--force` | Force removal even with uncommitted changes |

**Examples:**

```bash
# Remove a worktree (fails if there are uncommitted changes)
wt worktree remove /path/to/worktree

# Force removal even with uncommitted changes
wt worktree remove /path/to/worktree --force
```

**Safety:** By default, removal will fail if the worktree has uncommitted changes. Use `--force` to override this safety check.

---

## Configuration

wt uses YAML configuration files that are searched in the following order:

1. Path specified by `--config` flag
2. `.wt.yaml` in the current directory
3. `~/.config/wt/config.yaml` (XDG standard location)

### Configuration Options

```yaml
# Global settings
global:
  worktree_root: ~/dev/worktrees        # Base directory for worktrees

# Hooks run automatically after worktree operations
hooks:
  on_worktree_create:
    - run: "npm install"
      cwd: "{worktree_path}"             # Template expansion NOT YET IMPLEMENTED
      background: false                  # Run synchronously
      parallel: false                    # Can run with other parallel hooks

    - run: "npm run build"
      cwd: "{worktree_path}"             # Template expansion NOT YET IMPLEMENTED
      background: true                   # Run asynchronously
      parallel: true                     # Can run in parallel with other parallel hooks

  on_worktree_remove:
    - run: "rm -rf node_modules"
      cwd: "{worktree_path}"             # Template expansion NOT YET IMPLEMENTED

# Project-specific overrides using glob patterns
project_overrides:
  - match: "**/*rust*"                   # Matches projects with "rust" in path
    hooks:
      on_worktree_create:
        - run: "cargo fetch"
          cwd: "{worktree_path}"         # Template expansion NOT YET IMPLEMENTED
```

**IMPORTANT LIMITATIONS:**

- **Hook Template Expansion**: The `{worktree_path}` template is NOT yet implemented. When specifying `cwd`, use absolute paths or relative paths from your current directory. This feature is planned for a future release.

**NOT IMPLEMENTED - Future Features:**

The following configuration sections are documented but NOT implemented in the current version:
- `global.tmux_session_prefix` - Tmux integration is planned for a future release
- `tmux.*` section - All tmux configuration and session management is not yet available

---

## Common Workflows

### Creating a Feature Branch

```bash
# Create a new feature branch from main
wt worktree add feature/new-auth --base main

# The worktree is created at ~/dev/worktrees/feature/new-auth
# You can now work on the feature in isolation
```

### Working on a Bugfix

```bash
# Create a worktree for a bugfix
wt worktree add bugfix/crash-on-login

# Make your changes, commit them
# When done, remove the worktree
wt worktree remove ~/dev/worktrees/bugfix/crash-on-login
```

### Testing Multiple Branches

```bash
# Create worktrees for multiple feature branches
wt worktree add feature/a
wt worktree add feature/b
wt worktree add feature/c

# List all your worktrees
wt worktree list

# Test each feature in its isolated environment
```

### Parallel Development with Hooks

Configure hooks to automatically set up your environment:

```yaml
# .wt.yaml
hooks:
  on_worktree_create:
    - run: "npm install"
      cwd: "/absolute/path/to/project"  # Use absolute paths (template expansion not yet available)
      background: true
      parallel: true

    - run: "npm run build"
      cwd: "/absolute/path/to/project"  # Use absolute paths (template expansion not yet available)
      background: true
      parallel: true
```

Now when you run `wt worktree add feature/new`, dependencies are installed and the project is built automatically in the background.

**Note**: The `{worktree_path}` template expansion is not yet implemented. Use absolute paths or manage working directories manually for now.

### Cleaning Up Old Worktrees

```bash
# List all worktrees to find old ones
wt worktree list

# Remove worktrees you no longer need
wt worktree remove ~/dev/worktrees/old-feature
```

---

## Tips and Best Practices

1. **Use descriptive branch names** - Your worktree directory name will match your branch name
2. **Leverage hooks** - Automate repetitive tasks like dependency installation
3. **Use dry-run** - Preview changes before making them with `--dry-run`
4. **Filter your lists** - Use `--branches` or `--path` to quickly find specific worktrees
5. **Project-specific configs** - Use `.wt.yaml` for per-project hook configurations
