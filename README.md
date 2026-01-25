# wt - Git Worktree + Tmux CLI Tool

A high-level CLI tool for managing git worktrees with tmux integration and configurable hooks.

## Features

- **Worktree Management**: Create, list, and remove git worktrees
- **Tmux Integration**: Automatic session creation and attachment
- **Configurable Hooks**: Run commands after worktree operations (npm install, cargo build, etc.)
- **Project-Specific Configs**: Override settings per project using glob patterns

## Installation

```bash
go install github.com/user/wt/cmd/wt@latest
```

## Development

This project uses AI-friendly development tools with excellent community support:

### Prerequisites

- Go 1.20 or later
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

### Pre-commit Hooks (Optional)

```bash
pip install pre-commit
pre-commit install
```

### Development Commands

```bash
# Show all available commands
make help

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

## Configuration

Create a config file at `~/.config/wt/config.yaml` or `.wt.yaml` in your project:

```yaml
global:
  worktree_root: ~/dev/worktrees
  tmux_session_prefix: "wt-"

hooks:
  on_worktree_create:
    - run: "npm install"
      cwd: "{worktree_path}"
      background: false

tmux:
  layout: "main-vertical"
  window_name: "work"
  attach_on_create: true
```

See [configs/example.yaml](configs/example.yaml) for a complete example.

## Usage

```bash
# Create a new worktree
wt worktree create feature-branch

# List all worktrees
wt worktree list

# Remove a worktree
wt worktree remove /path/to/worktree

# Attach to a session
wt session attach wt-feature-branch

# Initialize config
wt config init
```

## Project Structure

```
wt/
├── cmd/wt/           # Main entry point
├── internal/
│   ├── cli/          # CLI commands (cobra)
│   ├── config/       # Configuration management
│   ├── git/          # Git operations
│   └── tmux/         # Tmux operations
├── pkg/executor/     # Subprocess execution
├── configs/          # Example configurations
└── tests/            # Integration tests
```

## Development Philosophy

This project is designed to be AI-friendly:
- **Well-known tools**: golangci-lint, cobra, testify - all with excellent documentation
- **Clear structure**: Standard Go project layout
- **Comprehensive tooling**: Linting, testing, hot-reload, CI/CD
- **Stable APIs**: Using mature libraries with active communities

## License

MIT
