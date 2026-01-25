# Architecture Diagram

## Package Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                            CLI Layer                            │
│                     (internal/cli/*.go)                         │
│                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │   root   │  │ worktree │  │ session  │  │  config  │       │
│  │   cmd    │  │   cmd    │  │   cmd    │  │   cmd    │       │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘       │
│       │             │             │             │               │
│       └─────────────┴─────────────┴─────────────┘               │
│                           │                                     │
│                           ▼                                     │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Service Layer                           │
│                    (internal/app/*.go)                          │
│                                                                   │
│  ┌──────────────────────┐  ┌──────────────────────┐            │
│  │  WorktreeService     │  │   SessionService     │            │
│  │                      │  │                      │            │
│  │  - List()            │  │  - EnsureWindow()    │            │
│  │  - Create()          │  │  - RenameWindow()    │            │
│  │  - Remove()          │  │  - SelectWindow()    │            │
│  │  - Setup()           │  │  - IsInside()        │            │
│  │  - Stack()           │  │                      │            │
│  └──────┬───────────────┘  └──────────┬───────────┘            │
│         │                             │                          │
│         └───────────┬─────────────────┘                          │
│                     │                                            │
└─────────────────────┼────────────────────────────────────────────┘
                      │
        ┌─────────────┼─────────────┐
        │             │             │
        ▼             ▼             ▼
┌─────────────┐ ┌──────────┐ ┌──────────┐
│     UI      │ │  Config  │ │ Executor │
│ (internal/  │ │(internal/│ │ (pkg/    │
│    ui/)     │ │  config/) │ │executor/) │
│             │ │          │ │          │
│  - fzf      │ │ - YAML   │ │ - Run()  │
│  - colors   │ │ - Validate│ │ - Run() │
│  - format   │ │ - Match  │ │          │
└──────┬──────┘ └────┬─────┘ └────┬─────┘
       │              │             │
       └──────────────┼─────────────┘
                      │
        ┌─────────────┼─────────────┐
        │             │             │
        ▼             ▼             ▼
┌─────────────┐ ┌──────────┐ ┌──────────┐
│     Git     │ │   Tmux   │ │   OS     │
│(internal/   │ │(internal/│ │          │
│    git/)    │ │  tmux/)  │ │(os/exec) │
│             │ │          │ │          │
│ - worktree  │ │ - session│ │ - spawn  │
│ - branch    │ │ - window │ │ - env    │
│ - remote    │ │ - attach │ │          │
└─────────────┘ └──────────┘ └──────────┘
```

## Data Flow: Worktree Creation

```
User: wt worktree add feature/auth
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│ CLI: worktreeCmd                                                │
│   - Parse flags (--setup, --no-window)                         │
│   - Load config                                                 │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│ WorktreeService.Create()                                        │
│                                                                  │
│  1. Validate branch name                                        │
│  2. git.ListBranches() - check if exists                        │
│  3. git.GetRepoName() - get project name                        │
│  4. Determine worktree path:                                    │
│     ~/.worktrees/<repo>/<branch>                                │
│  5. git.CreateWorktree(path, branch)                            │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│ SessionService.EnsureWindow()                                   │
│                                                                  │
│  1. tmux.IsInsideTmux()                                         │
│  2. format.GenerateWindowName(branch)                           │
│  3. tmux.ListWindows() - check exists                           │
│  4. If exists: tmux.SelectWindow()                              │
│     Else: tmux.NewWindow()                                      │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│ Executor: Run Hooks                                             │
│                                                                  │
│  config.GetRepoConfig(repoName)                                 │
│  for each setup_command:                                        │
│    executor.Run(cmd, worktree_path)                             │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│ Output                                                          │
│                                                                  │
│  - Print success message with icons/colors                      │
│  - Print worktree path for shell integration                    │
└─────────────────────────────────────────────────────────────────┘
```

## Data Flow: Stack Operation

```
User: wt worktree stack
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│ CLI: worktreeCmd (stack subcommand)                             │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│ WorktreeService.Stack()                                         │
│                                                                  │
│  1. git.GetCurrentBranch() → "feature/auth"                     │
│  2. ParseStackLevel("feature/auth") → 0 (not in stack)          │
│                                                                  │
│     If already in stack:                                        │
│       rootBranch = "feature/auth"                               │
│       nextLevel = currentLevel + 1                              │
│       Check: findHighestStackLevel()                            │
│       Validate: currentLevel == highest (no divergent)          │
│                                                                  │
│     If starting new stack:                                      │
│       rootBranch = currentBranch                                │
│       nextLevel = 1                                             │
│       Check: rootBranch__1 doesn't exist                        │
│                                                                  │
│  3. newBranch = "feature/auth__1"                               │
│  4. worktreePath = "~/.worktrees/<repo>/feature/auth__1"        │
│  5. git.CreateBranch("feature/auth__1", "feature/auth")         │
│  6. git.CreateWorktree(path, "feature/auth__1")                 │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│ SessionService: Handle Tmux Windows                             │
│                                                                  │
│  1. tmux.GetCurrentWindow() → "feature-auth"                    │
│  2. Rename to "feature-auth/0" (add stack suffix)               │
│  3. tmux.NewWindow("feature-auth/1", worktreePath)              │
└─────────────────────────────────────────────────────────────────┘
```

## Data Flow: Interactive Mode

```
User: wt (no args)
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│ CLI: rootCmd (no args)                                          │
│   - Check if TTY                                                │
│   - Check if fzf available                                      │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│ ui.Fzf.Select()                                                 │
│                                                                  │
│  1. git.ListWorktrees() → []Worktree                            │
│  2. format.FormatWorktrees(worktrees) → formatted strings       │
│  3. Pipe to fzf:                                                │
│     - Preview: git log in worktree                              │
│     - Keybindings:                                              │
│       Ctrl-A: wt add                                            │
│       Ctrl-D: wt remove                                         │
│       Ctrl-L: wt list                                           │
│       Ctrl-R: Refresh                                           │
│  4. Get selected worktree                                       │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│ SessionService.EnsureWindow()                                   │
│   - Prompt: create new or rename current?                       │
│   - Create/rename tmux window                                   │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│ Output                                                          │
│   - Print worktree path to stdout                               │
│   - Shell integration: cd "$(wt)"                               │
└─────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

### CLI Layer (`internal/cli/`)
- **Responsibility**: Parse arguments, invoke services, format output
- **Dependencies**: Service layer only
- **No**: Direct git/tmux operations, business logic

### Service Layer (`internal/app/`)
- **Responsibility**: Orchestrate operations across clients
- **Dependencies**: Git, Tmux, Config, Executor, UI
- **No**: CLI-specific concerns, direct user output

### UI Layer (`internal/ui/`)
- **Responsibility**: Interactive selection, formatting, colors
- **Dependencies**: fzf (external), TTY detection
- **No**: Business logic, service orchestration

### Format Layer (`internal/format/`)
- **Responsibility**: String transformations, naming conventions
- **Dependencies**: None
- **No**: I/O, external processes

### Clients (`internal/git/`, `internal/tmux/`)
- **Responsibility**: Wrap external tool CLI
- **Dependencies**: os/exec
- **No**: Business logic, formatting

### Config (`internal/config/`)
- **Responsibility**: Load, validate, match config
- **Dependencies**: YAML library
- **No**: Execute commands, modify git state

### Executor (`pkg/executor/`)
- **Responsibility**: Run subprocesses with timeout/parallel
- **Dependencies**: os/exec, context
- **No**: Business logic
