# wt - Git Worktree CLI Tool

A CLI tool for managing git worktrees with configurable hooks.

## Features

- **Worktree Management**: Add, list, and remove git worktrees
- **Configurable Hooks**: Run commands after worktree operations (npm install, cargo build, etc.)
- **Project-Specific Configs**: Override settings per project using glob patterns
- **Filtering**: Filter worktrees by branch name or path prefix

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

## Documentation

See [docs/usage.md](docs/usage.md) for detailed usage examples and command reference.

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
