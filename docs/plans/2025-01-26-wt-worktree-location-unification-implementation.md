# WT Worktree Location Unification Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Unify all worktree commands (add, remove, list, stack) to use the same config-based path resolution and delete unused `global.worktree_root`.

**Architecture:** The worktree.Service will now receive config and resolve paths based on `worktree.location` setting. Dedicated mode uses `worktree.dedicated_path/<branch>`, per-repo mode uses `<repo>/.worktrees/<branch>`. The git client no longer does auto-path generation.

**Tech Stack:**
- Go 1.21+
- `github.com/spf13/cobra` - CLI framework
- Go standard `testing` package
- `make` - Build automation

---

## Task 1: Remove WorktreeRoot from GlobalConfig Struct

**Files:**
- Modify: `internal/config/config.go:20-24`

**Step 1: Read the current GlobalConfig struct**

```bash
cat internal/config/config.go | head -25
```

Expected: See `WorktreeRoot string` field in GlobalConfig.

**Step 2: Remove WorktreeRoot field from GlobalConfig**

Delete this line from the struct:
```go
WorktreeRoot      string `yaml:"worktree_root"`
```

The struct should now only have `TmuxSessionPrefix`.

**Step 3: Verify code compiles**

```bash
go build ./internal/config/...
```

Expected: Clean build with no errors.

**Step 4: Remove WorktreeRoot from DefaultConfig**

Find and remove this line from `DefaultConfig()` function:
```go
WorktreeRoot:      "~/dev/worktrees",
```

**Step 5: Verify build still works**

```bash
go build ./internal/config/...
```

Expected: Clean build.

**Step 6: Commit**

```bash
git add internal/config/config.go
git commit -m "refactor(config): remove unused WorktreeRoot field

The global.worktree_root setting was never actually used by any
command. Removing to simplify config structure.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Remove WorktreeRoot Validation

**Files:**
- Modify: `internal/config/config.go:106-112`

**Step 1: Read current Validate function**

```bash
sed -n '106,112p' internal/config/config.go
```

Expected: See validation checking if `WorktreeRoot` is empty.

**Step 2: Remove the validation**

Delete the entire check:
```go
if c.Global.WorktreeRoot == "" {
    return fmt.Errorf("worktree_root cannot be empty")
}
```

The `Validate()` function should now just `return nil`.

**Step 3: Verify build**

```bash
go build ./internal/config/...
```

Expected: Clean build.

**Step 4: Update config test**

```bash
grep -n "WorktreeRoot" internal/config/config_test.go
```

If found, remove those test assertions.

**Step 5: Run config tests**

```bash
go test -v ./internal/config/...
```

Expected: All tests pass.

**Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "refactor(config): remove WorktreeRoot validation

No longer validates worktree_root since it's removed from config.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Add Config to Worktree Service

**Files:**
- Modify: `internal/worktree/service.go:11-24`

**Step 1: Read current Service struct**

```bash
sed -n '11,24p' internal/worktree/service.go
```

Expected: Service has `git git.GitClient` field only.

**Step 2: Add cfg field to Service struct**

Modify the struct to:
```go
type Service struct {
    git git.GitClient
    cfg *config.Config
}
```

**Step 3: Update NewService to accept config**

Find and modify the `NewService` function:
```go
// NewService creates a new worktree service
func NewService(gitClient git.GitClient, cfg *config.Config) (*Service, error) {
    if gitClient == nil {
        return nil, fmt.Errorf("gitClient cannot be nil")
    }
    if cfg == nil {
        cfg = config.DefaultConfig()
    }
    return &Service{
        git: gitClient,
        cfg: cfg,
    }, nil
}
```

**Step 4: Verify build**

```bash
go build ./internal/worktree/...
```

Expected: Clean build (tests may fail, that's next).

**Step 5: Commit**

```bash
git add internal/worktree/service.go
git commit -m "refactor(worktree): add config to Service constructor

Service now accepts config parameter for path resolution.
Defaults to DefaultConfig() if nil.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: Add ResolvePath Method to Worktree Service

**Files:**
- Modify: `internal/worktree/service.go:72`

**Step 1: Read current Add method to find insertion point**

```bash
sed -n '47,59p' internal/worktree/service.go
```

Expected: Add method ends around line 59.

**Step 2: Add ResolvePath method after Add method**

Insert this method at line 72 (after Add method, before Remove):
```go
// ResolvePath returns the worktree path for a branch.
// If explicitPath is provided, it's used as-is.
// Otherwise, path is resolved from config based on worktree.location setting.
func (s *Service) ResolvePath(ctx context.Context, branch string, explicitPath string) (string, error) {
    if explicitPath != "" {
        return explicitPath, nil
    }

    if s.cfg.Worktree.IsDedicated() {
        dedicatedPath := s.cfg.Worktree.GetDedicatedPath()
        // Expand ~ to home directory
        if strings.HasPrefix(dedicatedPath, "~/") {
            home, err := os.UserHomeDir()
            if err != nil {
                return "", fmt.Errorf("getting home directory: %w", err)
            }
            dedicatedPath = filepath.Join(home, dedicatedPath[2:])
        }
        return filepath.Join(dedicatedPath, branch), nil
    }

    // per-repo mode
    repoInfo, err := s.git.GetRepoInfo(ctx)
    if err != nil {
        return "", fmt.Errorf("getting repo info: %w", err)
    }
    return filepath.Join(repoInfo.RootPath, ".worktrees", branch), nil
}
```

**Step 3: Add missing imports**

Add to the import block at top of file:
```go
"os"
"path/filepath"
"strings"
```

**Step 4: Verify build**

```bash
go build ./internal/worktree/...
```

Expected: Clean build.

**Step 5: Commit**

```bash
git add internal/worktree/service.go
git commit -m "feat(worktree): add ResolvePath method for config-based paths

Resolves worktree paths based on worktree.location setting:
- dedicated mode: ~/worktrees/<branch>
- per-repo mode: <repo>/.worktrees/<branch>

Explicit paths are returned as-is.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: Update Add Method to Use ResolvePath

**Files:**
- Modify: `internal/worktree/service.go:47-59`

**Step 1: Read current Add method**

```bash
sed -n '47,59p' internal/worktree/service.go
```

**Step 2: Modify Add to resolve empty paths**

Replace the Add method body:
```go
// Add creates a new worktree
func (s *Service) Add(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
    // Resolve path if not specified
    if spec.Path == "" {
        resolvedPath, err := s.ResolvePath(ctx, spec.Branch, "")
        if err != nil {
            return nil, err
        }
        spec.Path = resolvedPath
    }

    if err := spec.Validate(); err != nil {
        return nil, fmt.Errorf("invalid spec: %w", err)
    }

    worktree, err := s.git.AddWorktree(ctx, spec)
    if err != nil {
        return nil, fmt.Errorf("adding worktree: %w", err)
    }

    return worktree, nil
}
```

**Step 3: Verify build**

```bash
go build ./internal/worktree/...
```

Expected: Clean build.

**Step 4: Commit**

```bash
git add internal/worktree/service.go
git commit -m "feat(worktree): use config-based path resolution in Add

Add now resolves paths from config when not explicitly provided.
Unifies behavior with stack command.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: Update Git Client to Require Path

**Files:**
- Modify: `internal/git/worktree.go:41-61`

**Step 1: Read current AddWorktree path logic**

```bash
sed -n '41,77p' internal/git/worktree.go
```

Expected: See lines 57-61 that auto-generate path from branch name.

**Step 2: Remove path auto-generation and require Path**

Delete these lines:
```go
path := spec.Path
if path == "" {
    // Auto-generate path from branch name
    path = filepath.Join(".", spec.Branch)
}
args = append(args, path)
```

Replace with:
```go
if spec.Path == "" {
    return nil, fmt.Errorf("spec.Path is required")
}
args = append(args, spec.Path)
```

**Step 3: Verify build**

```bash
go build ./internal/git/...
```

Expected: Clean build.

**Step 4: Commit**

```bash
git add internal/git/worktree.go
git commit -m "refactor(git): require Path in AddWorktree spec

Path auto-generation moved to worktree.Service.
Git client now requires Path to be set by caller.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: Update add.go CLI to Pass Config

**Files:**
- Modify: `internal/cli/add.go:33-60`

**Step 1: Read current add command Run function**

```bash
sed -n '33,60p' internal/cli/add.go
```

Expected: See service created without config.

**Step 2: Load config and pass to service**

Insert config loading before git client creation:
```go
ctx := context.Background()

// Load config
cfg, err := loadConfigForCommand()
if err != nil {
    Fatal("Failed to load config: %v", err)
}

gitClient, err := git.NewClient()
if err != nil {
    Fatal("Failed to create git client: %v", err)
}

svc, err := worktree.NewService(gitClient, cfg)
if err != nil {
    Fatal("Failed to create service: %v", err)
}
```

**Step 3: Verify build**

```bash
make build
```

Expected: Clean build.

**Step 4: Test manual behavior**

```bash
./bin/wt add --help
```

Expected: Help shows correctly, no errors.

**Step 5: Commit**

```bash
git add internal/cli/add.go
git commit -m "feat(add): pass config to worktree service

Add command now uses config-based path resolution.
Worktrees created in configured location instead of ./

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8: Update remove.go CLI to Pass Config and Accept Branch Names

**Files:**
- Modify: `internal/cli/remove.go:1-50`

**Step 1: Read current remove command**

```bash
cat internal/cli/remove.go
```

**Step 2: Add imports for strings and context**

Add to imports:
```go
"context"
"strings"
```

**Step 3: Load config and add branch name resolution**

Update the Run function:
```go
Run: func(cmd *cobra.Command, args []string) {
    path := args[0]

    ctx := context.Background()

    // Load config
    cfg, err := loadConfigForCommand()
    if err != nil {
        Fatal("Failed to load config: %v", err)
    }

    gitClient, err := git.NewClient()
    if err != nil {
        Fatal("Failed to create git client: %v", err)
    }

    svc, err := worktree.NewService(gitClient, cfg)
    if err != nil {
        Fatal("Failed to create service: %v", err)
    }

    // Resolve path if it looks like a branch name
    worktreePath := path
    if !strings.Contains(path, "/") {
        // Treat as branch name, resolve to actual path
        resolved, err := svc.ResolvePath(ctx, path, "")
        if err != nil {
            Fatal("Failed to resolve branch path: %v", err)
        }
        worktreePath = resolved
    }

    if err := svc.Remove(ctx, worktreePath, force); err != nil {
        Fatal("Failed to remove worktree: %v", err)
    }

    fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", worktreePath)
},
```

**Step 4: Update command help text**

Update Long text:
```go
Long: `Remove a worktree from the repository.

Accepts either a worktree path or branch name.
Branch names are resolved using the worktree.location config.

By default, this will fail if the worktree has uncommitted changes.
Use --force to remove it anyway.`,
```

**Step 5: Verify build**

```bash
make build
```

Expected: Clean build.

**Step 6: Commit**

```bash
git add internal/cli/remove.go
git commit -m "feat(remove): accept branch names and use config

Remove command now:
- Accepts branch names (resolves to path via config)
- Uses config-based path resolution

Branch names detected as values without '/' character.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9: Update list.go CLI to Pass Config

**Files:**
- Modify: `internal/cli/list.go:30-45`

**Step 1: Read current list command**

```bash
sed -n '30,50p' internal/cli/list.go
```

**Step 2: Load config and pass to service**

Update the Run function:
```go
Run: func(cmd *cobra.Command, args []string) {
    ctx := context.Background()

    // Load config
    cfg, err := loadConfigForCommand()
    if err != nil {
        Fatal("Failed to load config: %v", err)
    }

    gitClient, err := git.NewClient()
    if err != nil {
        Fatal("Failed to create git client: %v", err)
    }

    svc, err := worktree.NewService(gitClient, cfg)
    if err != nil {
        Fatal("Failed to create service: %v", err)
    }
```

**Step 3: Verify build**

```bash
make build
```

Expected: Clean build.

**Step 4: Commit**

```bash
git add internal/cli/list.go
git commit -m "feat(list): pass config to worktree service

List command now uses config for consistent path handling.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 10: Fix Worktree Service Tests

**Files:**
- Modify: `internal/worktree/service_test.go`

**Step 1: Read current tests**

```bash
cat internal/worktree/service_test.go
```

**Step 2: Update NewService calls**

Find all `worktree.NewService()` calls and add config parameter:
```go
svc, err := worktree.NewService(mockGit, cfg)
```

Each test will need a `cfg := config.DefaultConfig()` before NewService call.

**Step 3: Run worktree tests**

```bash
go test -v ./internal/worktree/...
```

Expected: All tests pass.

**Step 4: Commit**

```bash
git add internal/worktree/service_test.go
git commit -m "test(worktree): update tests for config parameter

All service tests now pass config to NewService.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 11: Fix Git Client Tests

**Files:**
- Modify: `internal/git/worktree_test.go`

**Step 1: Find tests that call AddWorktree**

```bash
grep -n "AddWorktree" internal/git/worktree_test.go
```

**Step 2: Update test specs to include Path**

Find any `WorktreeCreateSpec` that's missing Path and add:
```go
spec.Path = "/test/path",
```

**Step 3: Run git tests**

```bash
go test -v ./internal/git/...
```

Expected: All tests pass.

**Step 4: Commit**

```bash
git add internal/git/worktree_test.go
git commit -m "test(git): update tests for required Path field

All AddWorktree tests now provide required Path.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 12: Update CLI Tests

**Files:**
- Modify: `internal/cli/add_test.go`
- Modify: `internal/cli/list_test.go`
- Modify: `internal/cli/remove_test.go`

**Step 1: Check for broken tests**

```bash
go test -v ./internal/cli/... 2>&1 | grep -E "FAIL|undefined"
```

**Step 2: Fix any test issues**

Update test setups to mock config loading if needed.

**Step 3: Run all CLI tests**

```bash
go test -v ./internal/cli/...
```

Expected: All tests pass.

**Step 4: Commit**

```bash
git add internal/cli/*_test.go
git commit -m "test(cli): update tests for config changes

CLI tests updated for worktree service changes.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 13: Update README.md

**Files:**
- Modify: `README.md`

**Step 1: Find worktree_root references**

```bash
grep -n "worktree_root" README.md
```

**Step 2: Remove or replace references**

Remove `global.worktree_root` from config examples.
Replace with `worktree.location` and `worktree.dedicated_path`.

**Step 3: Verify markdown renders**

```bash
# No specific command, just visually check the file
```

**Step 4: Commit**

```bash
git add README.md
git commit -m "docs(readme): remove worktree_root, update config examples

Config now uses worktree.location and worktree.dedicated_path
for all worktree commands.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 14: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Find worktree_root references**

```bash
grep -n "worktree_root\|WorktreeRoot" CLAUDE.md
```

**Step 2: Update architecture section**

Remove WorktreeRoot mentions, update to describe unified path resolution.

**Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(claude): update architecture for unified path resolution

Document new config-based path resolution for all worktree commands.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 15: Update docs/usage.md

**Files:**
- Modify: `docs/usage.md`

**Step 1: Find relevant sections**

```bash
grep -n "worktree_root\|wt add" docs/usage.md | head -20
```

**Step 2: Update examples to reflect new behavior**

Show dedicated and per-repo modes in examples.

**Step 3: Commit**

```bash
git add docs/usage.md
git commit -m "docs(usage): update examples for config-based paths

Document worktree.location behavior for add/remove/list commands.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 16: Update configs/example.yaml

**Files:**
- Modify: `configs/example.yaml`

**Step 1: Read current example**

```bash
cat configs/example.yaml
```

**Step 2: Remove worktree_root line**

Delete the `worktree_root:` line from global section.

**Step 3: Commit**

```bash
git add configs/example.yaml
git commit -m "docs(example): remove worktree_root from example config

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 17: Run Full Test Suite

**Files:**
- All files

**Step 1: Run all tests**

```bash
make test
```

Expected: All tests pass (may have integration test failures due to environment).

**Step 2: Run linter**

```bash
make lint
```

Expected: No new lint errors.

**Step 3: Build binary**

```bash
make build
```

Expected: Clean build.

**Step 4: Manual smoke test**

```bash
# Test help still works
./bin/wt --help
./bin/wt add --help
./bin/wt remove --help
./bin/wt list --help
```

Expected: All help texts show correctly.

**Step 5: Commit**

```bash
git add .
git commit -m "test: verify all tests pass after unification

Full test suite passes with unified worktree path resolution.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 18: Clean Up Plan Files

**Files:**
- Issue tracking

**Step 1: Close the implementation bead**

```bash
bd close wt-diw --reason="Worktree location unification complete. All commands now use worktree.location config. global.worktree_root removed."
```

**Step 2: Sync beads**

```bash
bd sync --flush-only
```

**Step 3: Final commit**

```bash
git add .beads/
git commit -m "chore: close wt-diw bead after location unification

All worktree commands (add/remove/list/stack) now use unified
config-based path resolution.

Breaking changes:
- wt add now uses worktree.location instead of ./
- wt remove accepts branch names
- global.worktree_root removed (use worktree.dedicated_path)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Success Criteria

- [ ] `global.worktree_root` completely removed from code and docs
- [ ] `wt add` uses `worktree.location` config
- [ ] `wt remove` accepts branch names
- [ ] `wt list` passes config to service
- [ ] All unit tests pass
- [ ] No lint errors
- [ ] Documentation updated (README, CLAUDE.md, usage.md)
- [ ] Bead closed and synced

## Rollback Plan

If issues arise, revert the commits in reverse order:

```bash
git revert HEAD~18..HEAD
```

This will restore the previous behavior where:
- `wt add` creates worktrees at `./<branch>`
- `wt remove` only accepts paths
- `global.worktree_root` exists (unused)
