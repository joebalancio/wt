# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

## Build and Development Commands

```bash
# Build the binary
make build
# Output: bin/wt

# Run tests
make test                    # Run all tests
go test ./...               # Alternative
go test -v ./internal/git   # Run specific package tests
go test -run TestFoo        # Run specific test

# Linting and formatting
make lint                   # Run golangci-lint
make lint-fix              # Run linters with auto-fix
make fmt                   # Format code (go fmt + gofumpt if available)

# Development workflow
make check                 # Run all checks: fmt + lint + test

# Install development tools
make tools                 # Install golangci-lint, gofumpt, gox

# Other useful commands
make test-cover            # Generate coverage report (opens in browser)
make clean                 # Remove build artifacts
make install               # Install to GOPATH/bin
```


### wt config command

Programmatic configuration management for wt.

```bash
# Get a config value
wt config get worktree.location

# Set a config value (global config only)
wt config set worktree.location per-repo

# Unset/remove a config key (reverts to default)
wt config unset tmux.attach_on_create

# List all config values as YAML
wt config list

# Validate config (YAML syntax + schema validation)
wt config validate
```

**Supported keys:**
- `global.tmux_session_prefix` - Tmux session prefix
- `worktree.location` - Worktree location mode (dedicated/per-repo)
- `worktree.dedicated_path` - Path for dedicated mode
- `tmux.layout` - Default tmux layout
- `tmux.window_name` - Default tmux window name
- `tmux.attach_on_create` - Attach to tmux on worktree creation (boolean)
- `tmux.window_naming.max_length` - Maximum length for tmux window names (integer, 1-32)
- `tmux.window_naming.abbreviate_issue_id` - Abbreviate issue IDs in window names (boolean)

**Boolean values accepted:** `true`, `false`, `1`, `0`, `yes`, `no` (case-insensitive)

**Implementation files:**
- `internal/cli/cli_config_parser.go` - Dot-notation parser
- `internal/cli/cli_config_get.go` - Get command
- `internal/cli/cli_config_list.go` - List command
- `internal/cli/cli_config_set.go` - Set command
- `internal/cli/cli_config_unset.go` - Unset command
- `internal/cli/cli_config_validate.go` - Validate command
- `internal/config/config.go:ValidateSchema()` - Schema validation


### wt done command

Complete and remove a worktree by merging it into the current branch.

```bash
# Complete a worktree (merge and remove)
wt done /path/to/worktree feature-branch

# Force complete even with uncommitted changes
wt done --force /path/to/worktree feature-branch

# Dry run - show what would be done
wt done --dry-run /path/to/worktree feature-branch
```

**What it does:**
1. Validates the branch exists
2. Checks if worktree is dirty (fails unless --force is used)
3. Performs squash merge of feature branch into current branch
4. Creates merge commit with message "Merge <branch>"
5. Runs `on_worktree_done` hooks (if configured)
6. Removes the worktree
7. Deletes the feature branch
8. Runs `on_worktree_remove` hooks (if configured)

**Flags:**
- `--force` - Force completion even with uncommitted changes in the worktree
- `--dry-run` - Show what would be done without making changes (global flag)

**Hooks:**
Hooks are run at two points during the done workflow:

- `on_worktree_done` - Runs after merge but before worktree removal
  - Template variables: `{branch}`, `{worktree_path}`
  - Use case: Notifications, cleanup scripts, API calls

- `on_worktree_remove` - Runs after worktree removal
  - Template variables: `{branch}`, `{worktree_path}`
  - Note: worktree no longer exists when these hooks run

**Example hook configuration:**
```yaml
hooks:
  on_worktree_done:
    - run: echo "Branch {branch} completed" >> /tmp/wt.log
    - run: slack-cli -c "#dev" "Merged {branch}"
  on_worktree_remove:
    - run: echo "Cleaned up {branch}" >> /tmp/wt.log
```

**Implementation files:**
- `internal/cli/done.go` - CLI command implementation
- `internal/cli/done_test.go` - Command structure tests
- `internal/worktree/service.go:Done()` - Service layer logic
- `internal/worktree/service_test.go:TestService_Done()` - Unit tests
- `tests/done_integration_test.go` - Basic integration tests
- `tests/done_hooks_integration_test.go` - Hook integration tests
- `tests/done_conflict_integration_test.go` - Merge conflict tests
- `tests/done_dirty_integration_test.go` - Dirty worktree tests


## Architecture Overview

wt is a CLI tool that orchestrates three external tools: git, tmux, and user-defined hook commands. The architecture is organized around **client wrappers** for external tools and a **configuration-driven hook system**.

### High-Level Data Flow

```
User Command (CLI)
       ↓
Config Loading (YAML)
       ↓
┌─────────────────────────────────────┐
│  Worktree Creation Example Flow:    │
├─────────────────────────────────────┤
│ 1. Git Client → Create worktree     │
│ 2. Executor → Run post-create hooks │
│ 3. Tmux Client → Create session     │
│ 4. Tmux Client → Attach (optional)  │
└─────────────────────────────────────┘
```

### Package Structure and Relationships

```
cmd/wt/main.go          # Entry point - calls cli.Execute()
internal/
├── cli/               # Cobra commands (user-facing layer)
│   ├── root.go        # Global flags, command registration
│   ├── add.go         # Add worktree command
│   ├── list.go        # List worktrees command
│   ├── remove.go      # Remove worktree command
│   ├── session.go     # Session subcommands
│   ├── config.go      # Config subcommands
│   ├── stack.go       # Stack command (branch stacking)
│   ├── init.go        # Init command
│   ├── doctor.go      # Doctor command
│   └── setup.go       # Setup command (re-run hooks)
├── config/            # Configuration system
│   └── config.go      # YAML loading, validation, discovery
├── git/               # Git worktree operations
│   └── worktree.go    # Client pattern, porcelain parser
├── tmux/              # Tmux session operations
│   └── session.go     # Client pattern, list/create/attach/kill
├── spice/             # Git-spice client wrapper
│   └── client.go      # Branch stacking operations
├── stack/             # Stack management
│   ├── service.go     # Stack service logic
│   └── tree.go        # Tree formatting for display
└── worktree/          # Worktree service layer
    └── service.go     # Worktree operations
pkg/
├── domain/            # Domain models
│   └── worktree.go    # Worktree, StackBranch types
└── executor/          # Subprocess execution
    ├── executor.go    # General command execution
    └── hook_runner.go # Hook execution for setup automation
```

### Key Architectural Patterns

**1. Client Pattern for External Tools**

Both `internal/git` and `internal/tmux` use the same pattern:
- `NewClient()` - Finds the executable in PATH
- Methods wrap CLI commands (`git worktree add`, `tmux new-session`)
- Errors are wrapped with context: `fmt.Errorf("operation: %w", err)`

**2. Configuration Discovery Order (v3)**

Config is loaded in this priority order:
1. `--config` flag value (single config, no merging)
2. `.wt.yaml` at Git root (project-local, merges with global)
3. `~/.config/wt/config.yaml` (user-global)

When both project and global configs exist, they are merged:
- Project config overlays global config
- Scalars (strings, bools): project value wins
- Arrays (hooks): project array replaces global entirely
- Undefined fields: inherit from global

Implementation: `internal/config/config.go:FindConfig()` discovers Git root and merges configs.

**Worktree Location Configuration**

The `worktree.location` setting in config determines where worktrees are created:

- **dedicated mode** (default): Worktrees are created in a dedicated directory
  - Default path: `~/worktrees`
  - Custom path via `worktree.dedicated_path` in config
  - Example: `~/worktrees/feat/auth-api`

- **per-repo mode**: Worktrees are created within the repository
  - Path: `<repo-root>/.worktrees/<branch>`
  - Example: `/path/to/repo/.worktrees/feat/auth-api`

This configuration is used consistently across:
- `internal/worktree/service.ResolvePath()` - Resolves paths for worktree commands
- `internal/stack/service.getWorktreePath()` - Resolves paths for stack commands

Both services use the same logic: check `cfg.Worktree.IsDedicated()` and call `cfg.Worktree.GetDedicatedPath()` when in dedicated mode.

**3. Hook Execution Flow**

Hooks are defined in config with these fields:
- `run`: Command to execute
- `cwd`: Working directory (supports `{worktree_path}` template)
- `background`: Run asynchronously
- `parallel`: Can run with other parallel hooks

The executor (`pkg/executor/`) handles:
- Context cancellation
- Per-hook timeout (default 5 minutes)
- Parallel execution
- Output capture

**4. Global CLI State**

The root command maintains global state accessible via:
- `cli.GetDryRun()` - Check if --dry-run is set
- `cli.Verbose()` - Get verbosity level (count of -v flags)

**5. Process Replacement Pattern**

Tmux attach (`tmux.AttachSession()`) is special - it replaces the current process:
```go
cmd.Stdin = nil
cmd.Stdout = nil
cmd.Stderr = nil
// This replaces the current process, not a subprocess
```

### Important Implementation Details

**Git Worktree Parsing** (`internal/git/worktree.go`)

Git porcelain format parsing uses an index-based approach to avoid pointer aliasing bugs:
```go
var currentIndex int = -1
// When "worktree" line found:
worktrees = append(worktrees, Worktree{Path: value})
currentIndex = len(worktrees) - 1
// Subsequent lines update worktrees[currentIndex]
```

**Tmux Error Handling**

Tmux returns errors when:
- No server is running (gracefully handled, returns empty session list)
- Session doesn't exist (handled by HasSession before operations)

**Configuration Templates**

Hook commands support `{worktree_path}` template variable. This is expanded before execution in `pkg/executor/hook_runner.go`. The template is replaced with the actual worktree path when hooks are run.

### Development Notes

- **Adding new CLI commands**: Create file in `internal/cli/`, use `cli.RegisterCommand()` to register
- **Adding new hook types**: Extend `HooksConfig` in `internal/config/config.go`
- **Testing**: Tests use standard `testing` package; no external test framework
- **Linting**: `.golangci.yml` configures enabled linters (errcheck, staticcheck, revive, etc.)

### External Dependencies

- `github.com/spf13/cobra` - CLI framework
- `gopkg.in/yaml.v3` - YAML configuration parsing
- `github.com/aidarkhanov/nanoid` - Unique ID generation for branch suffixes
- `github.com/abhinav/git-spice` - Branch stacking (external dependency, invoked via CLI)

No other external dependencies. Git and tmux are invoked via CLI, not libraries.

### Stack Management (v2)

WT v2 adds stack management via git-spice integration:

- `internal/stack/service.go` - Stack operations using nanoid for unique suffixes
- `internal/stack/tree.go` - Tree formatting for stack hierarchy display
- `internal/spice/client.go` - Git-spice client wrapper
- `pkg/executor/hook_runner.go` - Hook execution for setup automation

Stack naming convention:
- Auto-suffix: `feat/auth` -> `feat/auth-xY7k` (4-char nanoid)
- Named suffix: `feat/auth` -> `feat/auth-api-k9P2`

The stack service integrates with:
- Git client for worktree operations
- Git-spice for branch stack management
- Config service for worktree path resolution
- Hook runner for post-create automation

### Tmux Window Integration (v2)

WT v2 adds automatic tmux window management:

- `internal/tmux/session.go` - Extended with window operations (CreateNewWindow, SelectWindow, KillWindow, CreateOrSelectWindow)
- `internal/tmux/window_naming_test.go` - Window naming logic tests
- `internal/tmux/window_test.go` - Window operation tests
- Smart naming: issue ID extraction, prefix/suffix abbreviation, stack numbering
- Integration points: `wt add`, `wt stack` create windows; `wt remove` cleans up
- Global `--no-tmux` flag to disable window creation per command

Window naming algorithm:
1. Extract issue IDs: `feature/nova-123` -> `nova-123`
2. Abbreviate prefixes: `bugfix` -> `fix`, `refactor` -> `ref`
3. Abbreviate suffixes: `auth-provider` -> `auth-p`
4. Truncate to 16 characters max
5. Stack numbering: root branch has no suffix, level 1 adds `/1`, level 2 adds `/2`

The tmux client integrates with:
- CLI commands (add, stack, remove) for automatic window management
- Config service for window naming preferences (MaxLength, AbbreviateIssueID)
