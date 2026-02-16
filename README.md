# wt - Git Worktree CLI Tool

A CLI tool for managing git worktrees with configurable hooks.

## Features

- **Worktree Management**: Add, list, and remove git worktrees
- **Configurable Hooks**: Run commands after worktree operations (npm install, cargo build, etc.)
- **Project-Local Configs**: Per-project settings via `.wt.yaml` at repository root
- **Filtering**: Filter worktrees by branch name or path prefix
- **Branch Stacking**: Integration with git-spice for stacked branch workflows

## Installation

### From source

```bash
git clone https://github.com/user/wt.git
cd wt
make build
sudo mv bin/wt /usr/local/bin/
```

### Using go install

```bash
go install github.com/user/wt@latest
```

### Optional: git-spice (for stacking)

git-spice enables branch stacking features. Install via:
- `cargo install git-spice`
- `brew install git-spice`

Run `wt init` after installing to configure automatically.

## Quick Start

```bash
# List worktrees in current repository
wt list

# Add a new worktree for a feature branch
# Creates worktree at configured location (default: ~/worktrees/<branch>)
wt add feature/login

# Add a worktree from a specific base branch
wt add feature/experimental --base main

# Remove a worktree and its branch
wt remove feature/login

# Force remove (uncommitted changes or unmerged branch)
wt remove feature/login --force

# Also delete remote branch
wt remove feature/login --force=remote

# Show help
wt --help
wt add --help
```

**Worktree Location**: By default, worktrees are created in `~/worktrees/`. Configure with `worktree.location` and `worktree.dedicated_path` (see [Configuration](#configuration) below).

## Stacking Features (v2)

WT v2 integrates with [git-spice](https://github.com/abhinav/git-spice) for branch stacking.

### Installation

1. Install git-spice:
   ```bash
   cargo install git-spice
   # or
   brew install git-spice
   ```

2. Initialize wt (auto-detects and configures git-spice):
   ```bash
   wt init
   ```

> **Note**: wt uses the "git-spice" command by default to avoid conflicts with Ghostscript's "gs". If you have the "gs" alias configured, ensure your config uses "git-spice".

### Basic Stacking Workflow

Stack commands require git-spice to be installed and configured. Run `wt doctor` to verify your setup.

```bash
# Create root branch
wt add feat/auth
cd ~/worktrees/feat/auth

# Stack on current (auto-suffix)
wt stack
# Creates: feat/auth-xY7k

# Stack with named suffix
wt stack api
# Creates: feat/auth-api-k9P2

# View stack hierarchy
wt stack list
```

### Configuration

Worktree location is configurable (see [Configuration](#configuration) below):

### Health Check

Run `wt doctor` to verify installation and dependencies.


## Tmux Integration

WT automatically creates tmux windows when you create worktrees while inside a tmux session.

### Features

- **Automatic window creation**: `wt add feat/auth` creates a new tmux window
- **Smart naming**: Branch names are abbreviated for readable window names
- **Stack support**: Stacked branches get numbered suffixes (feat/auth/1, feat/auth/2)
- **Automatic cleanup**: Windows are closed when removing worktrees

### Window Naming

| Branch | Window Name |
|--------|-------------|
| `feat/auth` | `feat/auth` |
| `feature/nova-123` | `nova-123` |
| `feature/api-fix` | `feat/a-f` |
| `feat/auth-xY7k` (stack level 1) | `feat/auth/1` |

### Disabling Tmux Integration

To skip window creation for a single command:

```bash
wt add feat/auth --no-tmux
wt stack --no-tmux
```

See [Tmux Windows Documentation](docs/tmux-windows.md) for details.

## Documentation

See [docs/usage.md](docs/usage.md) for detailed usage examples and command reference.

## Configuration

Configuration is loaded with **layered merging**:

1. **Global**: `~/.config/wt/config.yaml` - User-wide defaults
2. **Project**: `.wt.yaml` at repository root - Overrides global settings

**Merge behavior**: Project config overlays global config. Scalars (strings, bools) use project value; arrays (hooks) are replaced entirely.

Create a config file:

```yaml
# Git-spice configuration for branch stacking
spice:
  binary_path: ""  # Auto-detected via 'wt init', or set manually to e.g. "/usr/local/bin/git-spice"

worktree:
  location: dedicated      # "dedicated" or "per-repo"
  dedicated_path: ~/worktrees  # custom path for dedicated mode

hooks:
  on_worktree_create:
    - run: "npm install"
      cwd: "{worktree_path}"  # Supports template variable
```

### Git-Spice Configuration

The `spice.binary_path` setting specifies the git-spice binary location.
- Run `wt init` for automatic detection and configuration
- Or set manually: `binary_path: "/usr/local/bin/git-spice"`

**Note**: wt uses "git-spice" command by default to avoid conflicts with Ghostscript's "gs". If your git-spice is installed as "gs", run `wt init` to update your config automatically.

## Development

### Prerequisites

- Go 1.22 or later
- Git
- Make (optional, but recommended)

### Development Tools

```bash
# Install all development tools
make tools

# Or install individually:
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install mvdan.cc/gofumpt@latest
```

### Development Commands

```bash
# Format code
make fmt

# Run linters
make lint

# Run tests
make test

# Run tests with coverage report
make test-cover

# Build the binary
make build

# Run all checks
make check
```

## Project Structure

```
wt/
├── cmd/wt/           # Main entry point
├── internal/
│   ├── cli/          # CLI commands (cobra)
│   ├── config/       # Configuration management
│   ├── git/          # Git operations
│   └── worktree/     # Worktree service logic
├── pkg/              # Public packages
│   ├── domain/       # Domain models
│   └── executor/     # Subprocess execution
├── configs/          # Example configurations
└── docs/             # Documentation
```

## License

MIT
