# wt Usage Guide

This guide provides detailed usage examples for all wt commands.

## Table of Contents

- [Global Options](#global-options)
- [Worktree Commands](#worktree-commands)
  - [wt add](#wt-add)
  - [wt list](#wt-list)
  - [wt remove](#wt-remove)
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
wt -c /path/to/custom.yaml list

# Run in verbose mode
wt -v add feature-branch

# See what would happen without making changes
wt --dry-run remove /path/to/worktree
```

---

## Worktree Commands

### wt add

Add a new worktree for the specified branch.

**Synopsis:**

```bash
wt add <branch> [flags]
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
wt add feature/login

# Create a new branch from main and add worktree
wt add feature/auth --base main

# Create a worktree at a custom path
wt add hotfix-123 --path ~/projects/hotfix

# Create worktree without checking out files
wt add experiment --no-checkout
```

**How it works:**

1. The command ALWAYS creates a new branch with the specified name
2. The new branch is created from the base branch (default: current HEAD)
3. The worktree path is determined by your `worktree.location` configuration:
   - **Dedicated mode** (default): `worktree.dedicated_path/<branch>` (e.g., `~/worktrees/feature/login`)
   - **Per-repo mode**: `<repo>/.worktrees/<branch>`
4. Use `--path` to override the default location
5. If a branch with the same name already exists, the command will fail

---

### wt list

List all git worktrees in the current repository.

**Synopsis:**

```bash
wt list [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--branches <names>` | Filter by branch names (comma-separated) |
| `--path <prefix>` | Filter by path prefix |

**Examples:**

```bash
# List all worktrees
wt list

# Filter by specific branches
wt list --branches feature/login,feature/auth

# Filter by path prefix
wt list --path ~/projects/

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

### wt remove

Remove a worktree from the repository.

**Synopsis:**

```bash
wt remove <path> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--force` | Force removal even with uncommitted changes |

**Examples:**

```bash
# Remove a worktree (fails if there are uncommitted changes)
wt remove /path/to/worktree

# Force removal even with uncommitted changes
wt remove /path/to/worktree --force
```

**Safety:** By default, removal will fail if the worktree has uncommitted changes. Use `--force` to override this safety check.

---

## Configuration

wt uses YAML configuration files with a layered approach:

### Configuration Discovery (v3)

Configs are loaded and merged in this order:

1. `--config` flag value (single config, no merging)
2. `.wt.yaml` at Git root (project-local, merges with global)
3. `~/.config/wt/config.yaml` (user-global, XDG standard)

**Merge semantics when both project and global configs exist:**
- Project config overlays global config
- Scalars (strings, bools, numbers): project value wins
- Arrays (hooks): project array replaces global entirely
- Undefined fields: inherit from global

This allows team-wide defaults in `~/.config/wt/config.yaml` with project-specific overrides in `.wt.yaml` at the repository root.

### Configuration Options

```yaml
# Worktree location configuration
worktree:
  location: dedicated                    # "dedicated" or "per-repo"
  dedicated_path: ~/worktrees            # Used when location is "dedicated"

# Hooks run automatically after worktree operations
hooks:
  on_worktree_create:
    - run: "npm install"
      cwd: "{worktree_path}"             # Template expansion NOT YET IMPLEMENTED

  on_worktree_remove:
    - run: "rm -rf node_modules"
      cwd: "{worktree_path}"             # Template expansion NOT YET IMPLEMENTED
```

**Worktree Location Modes:**

- **Dedicated mode** (default): All worktrees are stored in a dedicated directory (`~/worktrees` by default). This keeps your worktrees separate from your repository.
- **Per-repo mode**: Worktrees are stored in `.worktrees/` subdirectory within each repository. This keeps worktrees close to the repository code.

**IMPORTANT LIMITATIONS:**

- **Hook Template Expansion**: The `{worktree_path}` template is NOT yet implemented. When specifying `cwd`, use absolute paths or relative paths from your current directory. This feature is planned for a future release.

**NOT IMPLEMENTED - Future Features:**

The following configuration sections are documented but NOT implemented in the current version:
- `global.tmux_session_prefix` - Tmux integration is planned for a future release
- `tmux.*` section - All tmux configuration and session management is not yet available


### wt config

Programmatic configuration management for wt.

**Synopsis:**

```bash
wt config <command> [arguments]
```

**Subcommands:**

| Command | Description |
|---------|-------------|
| `get <key>` | Get a config value |
| `list` | List all config values |
| `set <key> <value>` | Set a config value (global config only) |
| `unset <key>` | Remove a config key (global config only) |
| `validate` | Validate config (YAML + schema) |

**Supported Keys:**

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `global.tmux_session_prefix` | string | `wt-` | Prefix for tmux session names |
| `worktree.location` | string | `dedicated` | Worktree location mode (`dedicated` or `per-repo`) |
| `worktree.dedicated_path` | string | `~/worktrees` | Path for dedicated mode worktrees |
| `tmux.layout` | string | `main-vertical` | Default tmux layout |
| `tmux.window_name` | string | `work` | Default tmux window name |
| `tmux.attach_on_create` | bool | `true` | Attach to tmux session on worktree creation |
| `tmux.window_naming.max_length` | int | `16` | Maximum length for tmux window names |
| `tmux.window_naming.abbreviate_issue_id` | bool | `true` | Abbreviate issue IDs in window names |

**Examples:**

```bash
# Get a config value
wt config get worktree.location
# Output: dedicated

# Set a value
wt config set worktree.location per-repo
# Output: ✓ Updated worktree.location: per-repo in ~/.config/wt/config.yaml

# Validate config
wt config validate
# Output: ✓ Config is valid: ~/.config/wt/config.yaml
#         ✓ YAML syntax valid
#         ✓ Schema validation passed

# List all config values
wt config list
# Output: (full YAML config)

# Remove a key (reverts to default)
wt config unset worktree.dedicated_path
# Output: ✓ Removed worktree.dedicated_path from ~/.config/wt/config.yaml
```

**Boolean Values:**

When setting boolean values, the following inputs are accepted:
- `true`: `true`, `1`, `yes`, `on` (case-insensitive)
- `false`: `false`, `0`, `no`, `off` (case-insensitive)

```bash
wt config set tmux.attach_on_create false
wt config set tmux.attach_on_create yes   # Sets to true
```

**Error Handling:**

```bash
# Invalid enum value
wt config set worktree.location invalid
# Error: invalid value "invalid" for worktree.location
#        Valid values: dedicated, per-repo

# Invalid boolean
wt config set tmux.attach_on_create maybe
# Error: invalid boolean value: "maybe" (use: true, false, 1, 0, yes, no)

# Unsupported key
wt config set hooks.on_worktree_create "echo hi"
# Error: key "hooks.on_worktree_create" not supported for CLI manipulation
#        Edit config file directly to modify hooks
```

**Notes:**
- `set` and `unset` commands always modify the global config (`~/.config/wt/config.yaml`)
- For hooks, edit the config file directly (project-local `.wt.yaml` or global config)
- `get` and `list` respect the config discovery order (flag → local → global)

---
---

## Common Workflows

### Creating a Feature Branch

```bash
# Create a new feature branch from main
wt add feature/new-auth --base main

# The worktree is created based on your worktree.location config:
# - Dedicated mode (default): ~/worktrees/feature/new-auth
# - Per-repo mode: <repo>/.worktrees/feature/new-auth
# You can now work on the feature in isolation
```

### Working on a Bugfix

```bash
# Create a worktree for a bugfix
wt add bugfix/crash-on-login

# Make your changes, commit them
# When done, remove the worktree
wt remove ~/worktrees/bugfix/crash-on-login
# Or use branch name in dedicated mode:
wt remove bugfix/crash-on-login
```

### Testing Multiple Branches

```bash
# Create worktrees for multiple feature branches
wt add feature/a
wt add feature/b
wt add feature/c

# List all your worktrees
wt list

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

    - run: "npm run build"
      cwd: "/absolute/path/to/project"  # Use absolute paths (template expansion not yet available)
```

Now when you run `wt add feature/new`, dependencies are installed and the project is built automatically.

**Note**: The `{worktree_path}` template expansion is not yet implemented. Use absolute paths or manage working directories manually for now.

### Cleaning Up Old Worktrees

```bash
# List all worktrees to find old ones
wt list

# Remove worktrees you no longer need
wt remove ~/worktrees/old-feature
# Or use branch name in dedicated mode:
wt remove old-feature
```

---

## Tips and Best Practices

1. **Choose your worktree location mode** - Use `worktree.location: dedicated` to keep worktrees separate, or `worktree.location: per-repo` to keep them within each repository
2. **Use descriptive branch names** - Your worktree directory name will match your branch name
3. **Leverage hooks** - Automate repetitive tasks like dependency installation
4. **Use dry-run** - Preview changes before making them with `--dry-run`
5. **Filter your lists** - Use `--branches` or `--path` to quickly find specific worktrees
6. **Project-specific configs** - Use `.wt.yaml` for per-project hook configurations
