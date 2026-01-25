# Go Worktree Manager Design

**Date:** 2025-01-25
**Status:** Design Validated
**Scope:** MVP - Basic Worktree Operations (add, remove, list)

## Overview

A Go reimplementation of the `wt` bash script (~900 lines) focusing on maintainability, type safety, testability, and performance. The design uses a layered architecture with clear separation of concerns.

## Motivation

- **Type Safety & Reliability:** Eliminate bash scripting errors, add compile-time type checking
- **Testability:** Enable comprehensive unit tests, TDD workflow, mockable dependencies
- **Maintainability:** Better code organization, IDE support, easier to understand and modify
- **Performance:** Faster execution, potential for concurrent operations

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   CLI Layer (cobra)                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │   add    │  │  remove  │  │   list   │  ...     │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘          │
└───────┼────────────┼─────────────┼──────────────────┘
        │            │              │
┌───────┴────────────┴──────────────┴──────────────────┐
│                 Application Layer                    │
│  ┌──────────────────────────────────────────────┐   │
│  │         WorktreeService (orchestration)       │   │
│  └──────────────────────────────────────────────┘   │
└───────────────────────────────────────────────────────┘
        │
┌───────┴───────────────────────────────────────────────┐
│                  Domain Layer                         │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐ │
│  │   GitRepo   │  │  Worktree   │  │   Branch     │ │
│  │  (entity)   │  │  (entity)   │  │   (entity)   │ │
│  └─────────────┘  └─────────────┘  └──────────────┘ │
└───────────────────────────────────────────────────────┘
        │
┌───────┴───────────────────────────────────────────────┐
│               Infrastructure Layer                     │
│  ┌──────────────────┐  ┌──────────────────────────┐  │
│  │   GitClient      │  │   FileSystem             │  │
│  │   (exec wrapper) │  │   (os/fs operations)     │  │
│  └──────────────────┘  └──────────────────────────┘  │
└───────────────────────────────────────────────────────┘
```

## Package Structure

```
cmd/wt/                 # CLI entry point
├── main.go             # main() function
├── root.go             # Root command
├── add.go              # Add command
├── remove.go           # Remove command
└── worktree.go         # List command

internal/
├── git/                # Git client (infrastructure)
│   ├── client.go       # GitClient implementation
│   └── porcelain.go    # Porcelain parser
├── worktree/           # Service layer
│   └── service.go      # WorktreeService
└── config/             # Configuration (future)

pkg/domain/             # Shared domain entities
└── worktree.go         # Worktree, Branch, GitRepo, etc.
```

## Core Data Structures

```go
// Worktree represents a git worktree
type Worktree struct {
    Path     string    // Absolute path to worktree directory
    Branch   string    // Branch name (refs/heads/ prefix removed)
    Head     string    // Commit SHA or "detached"
    Bare     bool      // Is this a bare worktree
    Modified bool      // Has uncommitted changes
}

// WorktreeCreateSpec defines parameters for creating a worktree
type WorktreeCreateSpec struct {
    Branch    string   // Branch name (required)
    Base      string   // Base branch for new branches (optional)
    Path      string   // Custom path (optional, auto-generated if empty)
    Force     bool     // Force creation even if path exists
    Checkout  bool     // Whether to checkout the branch (default: true)
    Track     *string  // Remote branch to track (optional, pointer for nil vs empty)
}

// WorktreeFilter defines filters for listing worktrees
type WorktreeFilter struct {
    Branches    []string // Filter by branch names
    PathPrefix  string   // Filter by path prefix
    IncludeBare bool     // Include bare worktrees
}

// GitRepo represents the git repository context
type GitRepo struct {
    RootPath       string   // Absolute path to main worktree
    DefaultBranch  string   // Default branch (main/master)
    IsBare         bool     // Is this a bare repository
}

// Branch represents a git branch
type Branch struct {
    Name   string  // Short branch name
    SHA    string  // Commit SHA
    Remote *string // Remote name (nil for local-only)
}
```

## GitClient Interface

```go
type GitClient interface {
    ListWorktrees(ctx context.Context) ([]*domain.Worktree, error)
    AddWorktree(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
    RemoveWorktree(ctx context.Context, path string, force bool) error
    GetRepoInfo(ctx context.Context) (*domain.GitRepo, error)
    BranchExists(ctx context.Context, branch string) (bool, error)
}
```

**Key implementation decisions:**
- Uses `git worktree` CLI via `exec` (full git compatibility)
- Context propagation for cancellation
- Error wrapping with semantic context

## Service Layer

```go
type Service struct {
    git    GitClient
    repo   *domain.GitRepo
    config *Config
}

// Core methods
func (s *Service) List(ctx context.Context, filter *domain.WorktreeFilter) ([]*domain.Worktree, error)
func (s *Service) Add(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
func (s *Service) Remove(ctx context.Context, path string, force bool) error
```

**Service responsibilities:**
- Business logic (path generation, validation)
- Interface abstraction (depends on GitClient, not concrete type)
- Configuration handling
- Error context wrapping

## CLI Commands

```
wt                    # List worktrees (default)
wt add <branch>       # Add new worktree
  --base <branch>     # Base branch for new branches
  --path <path>       # Custom worktree path
  --force             # Force creation
  --track <remote>    # Remote branch to track

wt remove <path>      # Remove worktree
  --force             # Force removal

wt list               # List worktrees (explicit)
  --branches <list>   # Filter by branch names
  --path <prefix>     # Filter by path prefix
```

## Testing Strategy

### Porcelain Parser Tests (Unit)
- Table-driven tests for all porcelain format variations
- Single worktree, detached, bare, multiple worktrees

### Service Tests (Unit with Mocks)
- Use `gomock` to generate mocks for `GitClient`
- Test business logic in isolation
- Cover: new branch creation, existing branch, validation, filtering

### Integration Tests (Future)
- Use `testfixtures` or temporary git repositories
- Test actual git CLI interaction

### Test Structure Example
```go
func TestService_Add_NewBranch(t *testing.T) {
    t.Run("creates new branch from base", func(t *testing.T) {
        // Arrange: setup mock with expectations
        // Act: call svc.Add()
        // Assert: verify result and mock calls
    })
}
```

## Implementation Order

1. **Phase 1: Foundation**
   - Create package structure
   - Define domain entities
   - Implement porcelain parser (with tests)

2. **Phase 2: Git Client**
   - Implement `GitClient` interface
   - Add tests for porcelain parsing
   - Add integration tests with real git

3. **Phase 3: Service Layer**
   - Implement `WorktreeService`
   - Add unit tests with mocks
   - Implement path generation logic

4. **Phase 4: CLI Layer**
   - Setup cobra root command
   - Implement add, remove, list commands
   - Add integration tests

## Future Enhancements (Out of Scope for MVP)

- Interactive mode with fzf
- Tmux integration
- Stack mode (linear stacked branches)
- Hook system with YAML config
- Configuration file support
- Output formatting (JSON, table)

## Dependencies

```
github.com/spf13/cobra      # CLI framework
github.com/stretchr/testify # Assertions, test helpers
go.uber.org/mock            # Mock generation
```

No git library - uses `git` CLI via `os/exec`.
