# wt - Git Worktree CLI Tool

A CLI tool for managing git worktrees with configurable hooks.

## Features

- **Worktree Management**: Add, list, and remove git worktrees
- **Configurable Hooks**: Run commands after worktree operations (npm install, cargo build, etc.)
- **Project-Specific Configs**: Override settings per project using glob patterns
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

## Quick Start

```bash
# List worktrees in current repository
wt worktree list

# Add a new worktree for a feature branch
wt worktree add feature/login

# Add a worktree from a specific base branch
wt worktree add feature/experimental --base main

# Remove a worktree
wt worktree remove /path/to/worktree

# Show help
wt --help
wt worktree --help
wt worktree add --help
```

## Stacking Features (v2)

WT v2 integrates with [git-spice](https://github.com/abhinav/git-spice) for branch stacking.

### Installation

1. Install git-spice:
   ```bash
   cargo install git-spice
   # or
   brew install git-spice
   ```

2. Initialize wt:
   ```bash
   wt init
   ```

### Basic Stacking Workflow

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

Worktree location is configurable in `~/.config/wt/config.yaml`:

```yaml
worktree:
  location: dedicated      # "dedicated" or "per-repo"
  dedicated_path: ~/worktrees  # custom path for dedicated mode
```

### Health Check

Run `wt doctor` to verify installation and dependencies.

## Documentation

See [docs/usage.md](docs/usage.md) for detailed usage examples and command reference.

## Configuration

Create a config file at `~/.config/wt/config.yaml` or `.wt.yaml` in your project:

```yaml
global:
  worktree_root: ~/dev/worktrees

hooks:
  on_worktree_create:
    - run: "npm install"
      cwd: "/absolute/path/to/project"  # Use absolute paths (template expansion not yet available)
      background: false
```

See [configs/example.yaml](configs/example.yaml) for a complete example.

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
go install github.com/cosmtrek/air@latest
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

# Run with hot-reload
make dev

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
