# WT TMUX Integration Design

**Status:** Design Phase
**Author:** WT Team
**Created:** 2025-01-25
**Related:** [v2 Stacking Design](./2025-01-25-wt-v2-stacking-design.md)

## Overview

WT integrates with tmux to automatically create and manage windows for each worktree. When creating a worktree while in tmux, WT automatically creates a new window with a smart name and switches to it.

### Goals

1. **Automatic window creation** - Create tmux windows when adding worktrees
2. **Smart naming** - Abbreviated, readable window names
3. **Automatic cleanup** - Close windows when removing worktrees
4. **Stack support** - Numbered windows for stacked branches
5. **Zero configuration** - Works out of the box with existing sessions

### Non-Goals

- Session creation (users create sessions manually)
- Interactive mode (deferred to future)
- Window layout management (users customize via tmux config)
- Multi-server support (single machine only)

---

## Architecture

### Session Model

```
One session per repository (user-created)

Session: "my-repo"
├── Window: "feat/auth"              ← Branch: feat/auth
├── Window: "feat/api"               ← Branch: feat/api
├── Window: "nova-123/0"             ← Root branch
├── Window: "nova-123/1"             ← Stack level 1
└── Window: "nova-123/2"             ← Stack level 2
```

**Key principles:**
- WT does NOT create sessions
- WT assumes session already exists
- WT only creates/switches windows

---

## Window Creation Behavior

### Automatic Creation

```bash
# User is in tmux session "my-repo"
$ echo $TMUX
/tmp/tmux-1000/default,1234,5678

wt add feat/auth
# → Creates worktree: ~/worktrees/my-repo/feat/auth
# → Creates window: "feat/auth"
# → Switches to new window
# → Window CWD: ~/worktrees/my-repo/feat/auth
```

### Detection Logic

```go
func shouldCreateTmuxWindow(opts Options) bool {
    // Check $TMUX environment variable
    if os.Getenv("TMUX") == "" {
        return false  // Not in tmux
    }

    // Check --no-tmux flag
    if opts.NoTmux {
        return false
    }

    // Check tmux command exists
    if !tmuxInstalled() {
        return false
    }

    return true
}
```

### Window Creation Flow

```go
func createTmuxWindow(worktreePath, branch string) error {
    // 1. Generate window name from branch
    windowName := generateWindowName(branch)

    // 2. Check if window already exists in current session
    if windowExists(windowName) {
        // Switch to existing window
        return switchToWindow(windowName, worktreePath)
    }

    // 3. Create new window
    return createNewWindow(windowName, worktreePath)
}

func createNewWindow(name, path string) error {
    // tmux new-window -c <path> -n <name>
    cmd := exec.Command("tmux", "new-window",
        "-c", path,
        "-n", name)
    return cmd.Run()
}

func switchToWindow(name, path string) error {
    // tmux select-window -t <name>
    exec.Command("tmux", "select-window", "-t", name).Run()

    // tmux send-keys -t <name> "cd <path>" Enter
    cmd := exec.Command("tmux", "send-keys", "-t", name,
        fmt.Sprintf("cd %s", path), "Enter")
    return cmd.Run()
}
```

---

## Window Naming Convention

### Smart Abbreviation

Port logic from bash script (lines 133-190):

| Input Branch | Window Name | Logic |
|--------------|-------------|-------|
| `feat/auth` | `feat/auth` | Abbreviate prefix |
| `feature/nova-123` | `nova-123` | Extract issue ID |
| `feature/api-fix` | `feat/a-f` | First char of words |
| `bugfix/auth-providers` | `fix/auth-p` | Abbreviate prefix |
| `very-long-branch-name-here` | `very-long-br` | Truncate to 16 chars |

### Stack Window Naming

Stacked branches use numbered suffixes:

| Branch | Window Name | Pattern |
|--------|-------------|----------|
| `feat/auth` (root) | `feat/auth` | Root has no suffix |
| `feat/auth-xY7k` | `feat-auth/1` | First stack level |
| `feat/auth-xY7k-aB2m` | `feat-auth/2` | Second stack level |

**Numbering logic:**
- Root branch: no suffix
- First stacked branch: `/1`
- Second stacked branch: `/2`
- Based on position in git-spice stack, not branch name

### Implementation

```go
func generateWindowName(branch string) string {
    // 1. Try issue ID extraction: feature/nova-123 → nova-123
    if issue := extractIssueID(branch); issue != "" {
        return truncate(issue, 16)
    }

    // 2. Parse branch components
    parts := strings.Split(branch, "/")

    var prefix, suffix string
    if len(parts) >= 2 {
        prefix = abbreviatePrefix(parts[0])  // feature → feat
        suffix = parts[1]                     // auth-provider
    } else {
        suffix = branch
    }

    // 3. Abbreviate suffix: auth-provider → a-p
    abbreviated := abbreviateSuffix(suffix)

    // 4. Combine and truncate
    var result string
    if prefix != "" {
        result = fmt.Sprintf("%s/%s", prefix, abbreviated)
    } else {
        result = abbreviated
    }

    return truncate(result, 16)
}

func generateStackWindowName(branch string, stackLevel int) string {
    // Get root branch name (strip nanoid suffixes)
    root := getStackRoot(branch)

    // Generate base name
    baseName := generateWindowName(root)

    // Add stack level suffix
    return fmt.Sprintf("%s/%d", baseName, stackLevel)
}

func extractIssueID(branch string) string {
    // Match: prefix/word-number pattern
    // feature/nova-123 → nova-123
    // fix/PROJ-456 → PROJ-456
    re := regexp.MustCompile(`^[^/]+/([^\-]+-\d+)`)
    matches := re.FindStringSubmatch(branch)
    if len(matches) > 1 {
        return matches[1]
    }
    return ""
}

func abbreviatePrefix(prefix string) string {
    abbreviations := map[string]string{
        "feature":  "feat",
        "bugfix":   "fix",
        "hotfix":   "hot",
        "chore":    "chr",
        "refactor": "ref",
        "test":     "tst",
    }
    if abbr, ok := abbreviations[prefix]; ok {
        return abbr
    }
    // Default: first 4 chars
    return truncate(prefix, 4)
}

func abbreviateSuffix(suffix string) string {
    // Split on non-alphanumeric: auth-provider → auth provider
    words := regexp.MustCompile(`[^a-zA-Z0-9]+`).Split(suffix, -1)

    var result string
    for _, word := range words {
        if word == "" {
            continue
        }
        // If word is all digits, keep it
        if regexp.MustCompile(`^\d+$`).MatchString(word) {
            result += "-" + word
        } else {
            // Take first char
            result += "-" + string(word[0])
        }
    }

    // Remove leading dash
    if strings.HasPrefix(result, "-") {
        result = result[1:]
    }
    return result
}

func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen]
}
```

---

## Integration Points

### wt add

```bash
wt add feat/auth
# Internally:
if inTmux() && !opts.NoTmux {
    windowName := generateWindowName("feat/auth")
    createWindow(windowName, worktreePath)
}
```

### wt stack

```bash
# Current branch: feat/auth-xY7k
wt stack
# Creates: feat/auth-xY7k-aB2m
# Internally:
if inTmux() && !opts.NoTmux {
    stackLevel := getSpiceStackLevel()  // 2
    windowName := generateStackWindowName("feat/auth-xY7k-aB2m", stackLevel)
    createWindow(windowName, worktreePath)
}
```

### wt remove

```bash
wt remove feat/auth
# Internally:
if inTmux() {
    if currentWindowMatches("feat/auth") {
        tmux("kill-window", "-t", "feat-auth")
    }
}
```

---

## Flags

### Global Tmux Control

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--no-tmux` | bool | false | Skip tmux window creation |

### Examples

```bash
# Normal: creates window
wt add feat/auth

# Skip tmux
wt add feat/auth --no-tmux

# Stack with tmux
wt stack

# Stack without tmux
wt stack --no-tmux
```

---

## Configuration

### Optional Tmux Settings

```yaml
# ~/.config/wt/config.yaml
tmux:
  enabled: true           # Global on/off (default: true)
  auto_create: true       # Auto-create windows (default: true)
  window_naming:
    max_length: 16        # Max window name length
    abbreviate_issue_id: true  # Extract ISSUE-123 pattern
```

### Per-Repo Override

```yaml
# .wt.yaml
tmux:
  enabled: false          # Disable for this repo
```

---

## Error Handling

### Graceful Degradation

```go
func handleTmux(worktreePath, branch string) error {
    // 1. Check if in tmux
    if !isInTmux() {
        return nil  // Not an error, just skip
    }

    // 2. Check if tmux is installed
    if !tmuxInstalled() {
        log.Warn("tmux not found, skipping window creation")
        return nil
    }

    // 3. Try to create window
    if err := createWindow(worktreePath, branch); err != nil {
        log.Warnf("failed to create tmux window: %v", err)
        // Don't fail the whole operation
        return nil
    }

    return nil
}
```

### Common Errors

| Error | Handling |
|-------|----------|
| Tmux not installed | Warning, skip window creation |
| Not in tmux ($TMUX empty) | Skip silently |
| Window creation fails | Warning, continue anyway |
| Session doesn't exist | Warning, create in default session |

---

## Testing

### Unit Tests

```go
func TestGenerateWindowName(t *testing.T) {
    tests := []struct {
        branch     string
        want       string
    }{
        {"feat/auth", "feat/auth"},
        {"feature/nova-123", "nova-123"},
        {"feature/api-fix", "feat/a-f"},
        {"very-long-branch-name", "very-long-br"},
    }

    for _, tt := range tests {
        got := generateWindowName(tt.branch)
        assert.Equal(t, tt.want, got)
    }
}
```

### Integration Tests

```bash
# Test in tmux environment
tmux new-session -d -s test-repo
wt add feat/auth
# Verify: window "feat/auth" exists
# Verify: window CWD is correct
```

---

## Implementation Phases

### Phase 1: Foundation
- [ ] Add tmux detection (`$TMUX`, command check)
- [ ] Implement `--no-tmux` flag
- [ ] Add window naming logic

### Phase 2: Window Creation
- [ ] Implement `createTmuxWindow()`
- [ ] Handle existing windows (switch vs create)
- [ ] Integrate with `wt add`

### Phase 3: Stack Support
- [ ] Get stack level from git-spice
- [ ] Implement numbered stack windows
- [ ] Integrate with `wt stack`

### Phase 4: Window Cleanup
- [ ] Detect current window on remove
- [ ] Close window when removing worktree
- [ ] Handle edge cases (last window, etc.)

### Phase 5: Polish
- [ ] Add configuration options
- [ ] Error handling improvements
- [ ] Documentation

---

## Open Questions

1. **Session validation:** Should wt verify the session exists before creating windows?
   - **Decision:** No, let tmux handle errors

2. **Window existence check:** How to handle windows with same name in different sessions?
   - **Decision:** Only check current session

3. **Stack level detection:** How to get position in git-spice stack?
   - **Decision:** Parse `gs stack` output

4. **Max windows:** Any limit on windows per session?
   - **Decision:** No, tmux handles this

---

## Dependencies

### External Tools
```
tmux >= 1.9  (window creation)
gs   >= 0.7.0 (stack metadata)
```

### Go Modules
```
github.com/aidarkhanov/nanoid/v3  # Nanoid generation
```

---

## References

- [Tmux Manual](https://man.openbsd.org/tmux.1)
- [Git-Spice Documentation](https://abhinav.github.io/git-spice/)
- [Original Bash Script](~/projects/home/bin/wt)
- [v2 Stacking Design](./2025-01-25-wt-v2-stacking-design.md)
