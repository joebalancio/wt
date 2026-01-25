# TDD: Bash to Go Conversion Plan

## Overview

Convert the 912-line Bash `wt` script (`~/projects/home/bin/wt`) to a Go implementation in this repository.

## Source Analysis

### Bash Script Features

The Bash script provides:
1. **Interactive mode** (no args): fzf-based worktree selector with previews
2. **add**: Create new worktrees with branch selection
3. **setup**: Run setup-worktree script + project-specific commands from YAML config
4. **stack**: Create stacked branches (branch__1, branch__2, etc.)
5. **remove**: Remove worktrees interactively
6. **list**: Display all worktrees

### Key Integrations
- **Git**: worktree operations (list, create, remove, prune)
- **Tmux**: window creation, renaming, switching (one session, multiple windows model)
- **fzf**: interactive selection with previews
- **YAML config**: `~/.config/worktree/config.yaml` with repo-specific setup commands

## Existing Go Codebase State

### Current Structure
```
cmd/wt/main.go          # Entry point (ignored by git)
internal/
├── cli/               # Cobra commands
│   ├── root.go        # Global flags, command registration
│   ├── worktree.go    # Worktree command stub
│   ├── session.go     # Session command stub
│   └── config.go      # Config command stub
├── config/            # YAML configuration
│   └── config.go      # Has hooks, overrides (glob-based), tmux settings
├── git/               # Git operations
│   └── worktree.go    # ListWorktrees, CreateWorktree, RemoveWorktree, PruneWorktrees
├── tmux/              # Tmux operations
│   └── session.go     # Session ops only (ListSessions, HasSession, CreateSession, AttachSession, KillSession)
pkg/executor/
└── executor.go        # Subprocess execution with timeout/parallel support
```

### Gaps Identified

1. **Git client missing**:
   - List local/remote branches
   - Create branch from base
   - Detect current repo name from remote URL
   - Resolve current branch

2. **Tmux client missing**:
   - Window operations (CreateWindow, RenameWindow, SelectWindow)
   - IsInsideTmux() check
   - CurrentSession(), CurrentWindowID()

3. **Config mismatch**:
   - Uses glob-based overrides (`**/*rust*`)
   - No tilde expansion
   - Hook template `{worktree_path}` not implemented

4. **No orchestration layer**:
   - Services to coordinate git/tmux/config/executor
   - Stack branch naming logic
   - Window name generation from branch names

5. **No interactive UI**:
   - fzf wrapper
   - Formatted output with colors/icons

## Architecture Design

### Package Structure

```
internal/
├── cli/               # Cobra command handlers (thin)
├── config/            # Configuration loading/validation
├── git/               # Git client (expand)
├── tmux/              # Tmux client (expand)
├── format/            # NEW: String formatting, window names, icons
├── ui/                # NEW: Interactive selection, fzf wrapper, colors
├── app/               # NEW: Service layer (orchestration)
└── types/             # NEW: Shared types if needed
pkg/
└── executor/          # Command execution (expand for template expansion)
```

### Service Layer (`internal/app/`)

```go
// WorktreeService orchestrates worktree operations
type WorktreeService struct {
    git     *git.Client
    tmux    *tmux.Client
    config  *config.Config
    exec    *executor.Executor
}

// Methods
- List(ctx) ([]Worktree, error)
- Create(ctx, branch, baseBranch, opts) (*Worktree, error)
- Remove(ctx, path, opts) error
- Setup(ctx, path) error
- Stack(ctx) (*Worktree, error)
```

```go
// SessionService handles tmux session/window operations
type SessionService struct {
    tmux *tmux.Client
}

// Methods
- EnsureWindow(worktree, opts) error
- RenameWindow(name) error
- SelectWindow(name) error
- IsInside() bool
```

## Feature Breakdown

### Phase 1: Core Foundation (Non-Interactive)

#### 1.1 Expand Git Client (`internal/git/`)
```go
// Add to git.Client
- ListBranches(ctx, opts) ([]Branch, error)
  // opts: localsOnly, remotesOnly, mergedOnly
  
- CreateBranch(ctx, name, baseBranch) error
  
- GetCurrentBranch(ctx) (string, error)
  
- GetRepoName(ctx) (string, error)
  // Tries: origin -> upstream -> folder basename
  
- GetDefaultBranch(ctx) (string, error)
  // Returns refs/remotes/origin/HEAD or "main"
  
- BranchExists(ctx, name) bool

- FindHighestStackLevel(ctx, rootBranch) (int, error)
  // Finds max N for branches matching root__N pattern
```

#### 1.2 Expand Tmux Client (`internal/tmux/`)

```go
// Add to tmux.Client
- IsInsideTmux() bool
  // Checks TMUX env var

- GetCurrentSession() (Session, error)
- GetCurrentWindowID() (string, error)

- NewWindow(session, name, path) error
- RenameWindow(windowID, name) error
- SelectWindow(windowID) error
- KillWindow(windowID) error

- ListWindows(session) ([]Window, error)
```

#### 1.3 Format Package (`internal/format/`)

```go
// Window name generation
func GenerateWindowName(branch string) string {
    // Rules:
    // 1. Try to extract linear issue ID: feature/{word-number} -> {word-number}
    // 2. Otherwise, create abbreviated name:
    //    - Abbreviate prefix (feature->feat, bugfix->fix, etc.)
    //    - Abbreviate suffix (first letter of each word)
    //    - Max 16 characters
    //    - Stack suffix: branch__N -> branch/N
}

// Icons (Unicode)
const (
    IconWorktree = "\U0001F333"  // 🌳
    IconBranch   = "\U0001F334"  // 🌿
    IconAdd      = "\U00002795"  // ➕
    IconRemove   = "\U0001F5D1"  // 🗑️
    IconList     = "\U0001F4CB"  // 📋
    IconStack    = "\U0001F4DA"  // 📚
    IconSetup    = "\U00002699"  // ⚙️
)
```

#### 1.4 Config Updates

```go
// Add to config package
- ExpandTilde(path string) string
- ApplyTemplate(cmd string, vars map[string]string) string
  // Expands {worktree_path}, etc.

// New config structure
type RepoConfig struct {
    SetupCommands []string `yaml:"setup_commands"`
}

type Config struct {
    Repos map[string]RepoConfig `yaml:"repos"`
    // ... existing fields
}

// Repo matching by name (not glob)
func (c *Config) GetRepoConfig(repoName string) (RepoConfig, bool)
```

### Phase 2: Service Layer

#### 2.1 WorktreeService Implementation

```go
func (s *WorktreeService) Create(ctx context.Context, branch, baseBranch string, opts CreateOpts) (*Worktree, error) {
    // 1. Validate inputs
    // 2. Detect/create branch
    // 3. Determine worktree path
    // 4. Create worktree via git client
    // 5. Run on_worktree_create hooks if configured
    // 6. Handle tmux window (if in tmux and opts.CreateWindow)
}

func (s *WorktreeService) Stack(ctx context.Context) (*Worktree, error) {
    // 1. Get current branch
    // 2. Parse stack level (branch__N)
    // 3. Determine next level
    // 4. Validate not creating divergent stack
    // 5. Create new branch from current
    // 6. Create worktree
    // 7. Update tmux windows
}
```

#### 2.2 SessionService Implementation

```go
func (s *SessionService) EnsureWindow(worktree Worktree, branch string, prompt bool) error {
    // 1. Check if inside tmux
    // 2. Generate window name from branch
    // 3. Check if window exists
    // 4. If prompt: ask user (new vs rename)
    // 5. Create or rename window
}
```

### Phase 3: Interactive UI

#### 3.1 Fzf Wrapper (`internal/ui/fzf.go`)

```go
type Fzf struct {
    path string
}

type SelectOpts struct {
    Prompt     string
    Header     string
    Preview    string
    Multi      bool
    Query      string
    Keybinds   map[string]string
}

func (f *Fzf) Select(items []string, opts SelectOpts) (string, error) {
    // Pipes items to fzf stdin, returns selected item
}

func (f *Fzf) Available() bool {
    // Checks if fzf is in PATH
}
```

#### 3.2 Colors (`internal/ui/color.go`)

```go
func SupportsColor() bool {
    // Checks TTY and NO_COLOR env
}

type Color struct {
    code string
}

var (
    Red    = Color{code: "\033[0;31m"}
    Green  = Color{code: "\033[0;32m"}
    Yellow = Color{code: "\033[1;33m"}
    // ...
)

func (c Color) Sprintf(format string, args ...interface{}) string
func (c Color) Sprint(args ...interface{}) string
```

### Phase 4: CLI Commands

#### 4.1 Worktree Commands

```bash
wt worktree list                    # List all worktrees
wt worktree add <branch> [base]     # Create worktree
  --setup, -s                       # Run setup after create
  --no-window                       # Skip tmux window creation
  
wt worktree remove [path]           # Remove worktree
  --force, -f                       # Skip confirmation
  
wt worktree stack                   # Create stacked worktree
wt worktree setup                   # Run setup in current worktree
```

#### 4.2 Interactive Mode

```bash
wt                                   # Interactive selection
  --print-cd                         # Print cd command
  --shell                            # Spawn shell in worktree
```

#### 4.3 Shell Integration

```bash
wt --shell-integration               # Generate shell function
# User adds to .bashrc/.zshrc:
# eval "$(wt --shell-integration)"
```

## Test Strategy

### Unit Tests

1. **Git client**: Mock exec.Command, test output parsing
2. **Tmux client**: Mock exec.Command, test command building
3. **Format package**: Test window name generation with various branch patterns
4. **Config**: Test tilde expansion, template expansion, repo matching

### Integration Tests

1. **WorktreeService**: Use test git repository, test create/list/remove
2. **SessionService**: Use test tmux server (docker or isolated)

### CLI Tests

1. **Command execution**: Test each subcommand with various flags
2. **Error handling**: Test error messages and exit codes

## Implementation Order

### Milestone 1: Core Functionality (Non-Interactive)
1. Expand git client (branches, repo name detection)
2. Expand tmux client (windows, inside check)
3. Create format package (window names, icons)
4. Update config (repo matching, templates)
5. Create service layer (WorktreeService, SessionService)
6. Implement `wt worktree list|add|remove`
7. Write tests

### Milestone 2: Advanced Features
1. Implement stack logic
2. Implement setup command (hooks + external setup-worktree)
3. Implement `wt worktree stack|setup`
4. Write tests

### Milestone 3: Interactive UI
1. Create fzf wrapper
2. Create color package
3. Implement interactive `wt` command
4. Implement preview pane
5. Write tests

### Milestone 4: Polish
1. Shell integration helper
2. Error message improvements
3. Documentation (man pages, examples)
4. Performance optimization

## Configuration Changes

### Old Config (Bash)
```yaml
repos:
  my-project:
    setup_commands:
      - "npm install"
```

### New Config (Go)
```yaml
global:
  worktree_root: ~/dev/worktrees
  tmux_session_prefix: "wt-"

repos:
  my-project:
    setup_commands:
      - run: "npm install"
        cwd: "{worktree_path}"

hooks:
  on_worktree_create:
    - run: "echo 'Worktree created'"
  
tmux:
  layout: "main-vertical"
  window_name: "work"
  attach_on_create: true
```

## Key Decisions

### Shell Integration
- **Decision**: Print path to stdout, user wraps in shell function
- **Rationale**: Go cannot change parent shell's directory
- **Alternative**: `wt --shell` spawns child shell

### Tmux Model
- **Decision**: One session with multiple windows
- **Rationale**: Matches Bash script behavior
- **Windows**: Named from branch, max 16 chars

### Interactive Mode
- **Decision**: Use external fzf
- **Rationale**: Faster to implement, familiar UX
- **Fallback**: If fzf unavailable, show list + require explicit args

### Stack Naming
- **Decision**: Use `__` separator (branch__1, branch__2)
- **Rationale**: Matches Bash script, unlikely to conflict
- **Window names**: Use `/` separator (branch/1, branch/2)

## Open Questions

1. Should we support `setup-worktree` as external command or integrate its functionality?
2. Should we support path-based config overrides in addition to repo-name matching?
3. Should stack prevent divergent branches or allow them with warning?
4. Should we support `--detach` flag for worktrees without tmux?

## Success Criteria

- [ ] All Bash script commands have Go equivalents
- [ ] Interactive mode works with fzf
- [ ] Tmux windows are created/renamed correctly
- [ ] Stack operations work (branch__N pattern)
- [ ] Setup commands run from config
- [ ] Tests pass (unit + integration)
- [ ] Shell integration documented
- [ ] Performance comparable to Bash (<100ms for most operations)
