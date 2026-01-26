# WT Worktree Location Unification Design

> **Status:** Proposed
> **Issue:** wt-diw
> **Created:** 2025-01-26

## Problem

The codebase has inconsistent worktree path resolution:

| Command | Current Behavior |
|---------|------------------|
| `wt stack` | Uses `worktree.location` & `worktree.dedicated_path` ✅ |
| `wt add` | Ignores config, uses `./<branch>` ❌ |
| `wt remove` | Only accepts explicit path ❌ |
| `global.worktree_root` | Defined but never used ❌ |

This creates confusion and inconsistent user experience.

## Goal

Unify all worktree commands to use the same config-based path resolution:
- **Dedicated mode**: `~/worktrees/<branch>` (configurable)
- **Per-repo mode**: `<repo>/.worktrees/<branch>`

Delete unused `global.worktree_root` entirely.

## Current vs Target Behavior

### Before
```bash
# Config has global.worktree_root (unused) and worktree.location (partial)
global.worktree_root: ~/dev/worktrees  # never worked
worktree.location: dedicated
worktree.dedicated_path: ~/worktrees   # only wt stack uses this

wt add feat/login      # creates ./feat/login/
wt stack feat/login    # creates ~/worktrees/feat/login/  # inconsistent!
```

### After
```bash
# Simple, unified config
worktree.location: dedicated
worktree.dedicated_path: ~/worktrees

wt add feat/login      # creates ~/worktrees/feat/login/
wt stack feat/login    # creates ~/worktrees/feat/login/  # consistent!
wt remove feat/login   # removes by branch name (new!)
```

## Implementation Changes

### 1. Config Cleanup (`internal/config/config.go`)

**Remove field:**
```go
type GlobalConfig struct {
-   WorktreeRoot      string `yaml:"worktree_root"`  // DELETE
    TmuxSessionPrefix string `yaml:"tmux_session_prefix"`
}
```

**Remove validation:**
```go
func (c *Config) Validate() error {
-   if c.Global.WorktreeRoot == "" {
-       return fmt.Errorf("worktree_root cannot be empty")
-   }
    return nil
}
```

**Update defaults:**
```go
func DefaultConfig() *Config {
    return &Config{
        Global: GlobalConfig{
-           WorktreeRoot:      "~/dev/worktrees",  // DELETE
            TmuxSessionPrefix: "wt-",
        },
        // rest unchanged
    }
}
```

### 2. Worktree Service Refactor (`internal/worktree/service.go`)

**Add config to service:**
```go
type Service struct {
    git git.GitClient
+   cfg *config.Config
}

-   func NewService(gitClient git.GitClient) (*Service, error) {
+   func NewService(gitClient git.GitClient, cfg *config.Config) (*Service, error) {
    if cfg == nil {
        cfg = config.DefaultConfig()
    }
    return &Service{git: gitClient, cfg: cfg}, nil
}
```

**Add path resolution method:**
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
            home, _ := os.UserHomeDir()
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

**Update Add method to use resolved path:**
```go
func (s *Service) Add(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
+   // Resolve path if not specified
+   if spec.Path == "" {
+       resolvedPath, err := s.ResolvePath(ctx, spec.Branch, "")
+       if err != nil {
+           return nil, err
+       }
+       spec.Path = resolvedPath
+   }

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

### 3. Git Client Simplification (`internal/git/worktree.go`)

**Require Path to be set:**
```go
func (c *Client) AddWorktree(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
    if err := spec.Validate(); err != nil {
        return nil, fmt.Errorf("invalid spec: %w", err)
    }

+   if spec.Path == "" {
+       return nil, fmt.Errorf("spec.Path is required")
+   }

    args := []string{"worktree", "add"}

    if spec.Force {
        args = append(args, "--force")
    }

    // Always create a new branch with -b flag
    args = append(args, "-b", spec.Branch)

-   path := spec.Path
-   if path == "" {
-       // Auto-generate path from branch name
-       path = filepath.Join(".", spec.Branch)
-   }
-   args = append(args, path)
+   args = append(args, spec.Path)

    // ... rest unchanged
}
```

### 4. CLI Updates

**`add.go` - Pass config to service:**
```go
+   // Load config
+   cfg, err := loadConfigForCommand()
+   if err != nil {
+       Fatal("Failed to load config: %v", err)
+   }

    gitClient, err := git.NewClient()
    if err != nil {
        Fatal("Failed to create git client: %v", err)
    }

-   svc, err := worktree.NewService(gitClient)
+   svc, err := worktree.NewService(gitClient, cfg)
    if err != nil {
        Fatal("Failed to create service: %v", err)
    }

    spec := domain.WorktreeCreateSpec{
        Branch:   branch,
        Base:     base,
        Path:     path,  // Empty string -> config resolves
        Force:    force,
        Checkout: !noCheckout,
    }
```

**`remove.go` - Accept branch names:**
```go
+   // Load config
+   cfg, err := loadConfigForCommand()
+   if err != nil {
+       Fatal("Failed to load config: %v", err)
+   }

    gitClient, err := git.NewClient()
    if err != nil {
        Fatal("Failed to create git client: %v", err)
    }

-   svc, err := worktree.NewService(gitClient)
+   svc, err := worktree.NewService(gitClient, cfg)
    if err != nil {
        Fatal("Failed to create service: %v", err)
    }

+   // Resolve path if it looks like a branch name
+   worktreePath := path
+   if !strings.Contains(path, "/") {
+       // Treat as branch name, resolve to actual path
+       resolved, err := svc.ResolvePath(ctx, path, "")
+       if err != nil {
+           Fatal("Failed to resolve branch path: %v", err)
+       }
+       worktreePath = resolved
+   }

    ctx := context.Background()

-   if err := svc.Remove(ctx, path, force); err != nil {
+   if err := svc.Remove(ctx, worktreePath, force); err != nil {
        Fatal("Failed to remove worktree: %v", err)
    }
```

**`list.go` - Pass config to service:**
```go
+   cfg, err := loadConfigForCommand()
+   if err != nil {
+       Fatal("Failed to load config: %v", err)
+   }

    gitClient, err := git.NewClient()
    if err != nil {
        Fatal("Failed to create git client: %v", err)
    }

-   svc, err := worktree.NewService(gitClient)
+   svc, err := worktree.NewService(gitClient, cfg)
    if err != nil {
        Fatal("Failed to create service: %v", err)
    }
```

### 5. Documentation Updates

**README.md:**
- Remove `global.worktree_root` from config examples
- Update to show `worktree.location` and `worktree.dedicated_path`

**CLAUDE.md:**
- Update architecture section to reflect unified path resolution
- Remove worktree_root mentions

**docs/usage.md:**
- Update examples to show new default behavior
- Document both dedicated and per-repo modes

**configs/example.yaml:**
- Remove `global.worktree_root`

## File Summary

| File | Action | Lines Changed |
|------|--------|---------------|
| `internal/config/config.go` | Remove WorktreeRoot, update validation/defaults | ~-10 lines |
| `internal/worktree/service.go` | Add config, Add ResolvePath(), update Add() | ~+40 lines |
| `internal/git/worktree.go` | Require Path, remove auto-generation | ~-5 lines |
| `internal/cli/add.go` | Pass config to service | ~+4 lines |
| `internal/cli/remove.go` | Pass config, resolve branch names | ~+10 lines |
| `internal/cli/list.go` | Pass config to service | ~+4 lines |
| `README.md` | Remove worktree_root references | ~-5 lines |
| `CLAUDE.md` | Update architecture docs | ~-10 lines |
| `docs/usage.md` | Update usage examples | ~-20 lines |
| `configs/example.yaml` | Remove worktree_root | ~-1 line |

**Total:** ~103 lines changed across 10 files.

## Edge Cases

1. **Tilde expansion in paths**
   - `~/worktrees` needs expansion to `/home/user/worktrees`
   - Handle in `ResolvePath()` method

2. **Explicit `--path` always wins**
   - User provides `--path` → use it, ignore config
   - No path provided → resolve from config

3. **Per-repo mode when not in a git repo**
   - `GetRepoInfo()` will fail
   - Return clear error message

4. **Branch name vs path ambiguity in `wt remove`**
   - If contains `/` → treat as path
   - If no `/` → treat as branch name, resolve via config

## Success Criteria

- [ ] `global.worktree_root` completely removed from code
- [ ] `wt add` uses `worktree.location` config
- [ ] `wt remove` accepts branch names
- [ ] `wt list` passes config to service
- [ ] All tests pass
- [ ] Documentation updated
- [ ] No references to WorktreeRoot remain

## Breaking Changes

This is a breaking change (pre-1.0):
- `wt add feat/login` now creates `~/worktrees/feat/login/` instead of `./feat/login/`
- Users with existing configs using `global.worktree_root` will need to update to `worktree.dedicated_path`
