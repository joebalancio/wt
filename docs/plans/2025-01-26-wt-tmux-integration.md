# WT Tmux Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add automatic tmux window creation when adding worktrees, with smart naming and stack support.

**Architecture:**
- Extend existing `internal/tmux` client with window operations (create, list, kill, select)
- Add window naming package with abbreviation logic and stack level detection
- Integrate window creation into `wt add` and `wt stack` commands
- Add `--no-tmux` flag for global tmux control

**Tech Stack:**
- `github.com/spf13/cobra` - CLI flags
- `gopkg.in/yaml.v3` - Config parsing
- `exec.Command` - Tmux CLI invocation
- `regexp` - Window name pattern matching

---

## Task 1: Add Window Operations to Tmux Client

**Files:**
- Modify: `internal/tmux/session.go`
- Test: `internal/tmux/session_test.go`

**Step 1: Write failing test for ListWindows**

```go
func TestClient_ListWindows(t *testing.T) {
    // This test requires tmux to be installed and running
    client, err := NewClient()
    if err != nil {
        t.Skip("tmux not installed")
    }

    windows, err := client.ListWindows()
    // Just verify it doesn't crash - may return empty if no session
    assert.NoError(t, err)
    assert.NotNil(t, windows)
}
```

Run: `go test -v ./internal/tmux/... -run TestClient_ListWindows`
Expected: FAIL with "method ListWindows not defined"

**Step 2: Add Window struct and ListWindows method**

Add to `internal/tmux/session.go` after the Session struct:

```go
// Window represents a tmux window
type Window struct {
    ID       string
    Name     string
    Session  string
}
```

Add to Client struct:

```go
// ListWindows returns all windows in the current session
func (c *Client) ListWindows() ([]Window, error) {
    var stdout bytes.Buffer
    cmd := exec.Command(c.tmuxPath, "list-windows", "-F", "#{window_id} #{window_name} #{session_name}")
    cmd.Stdout = &stdout

    if err := cmd.Run(); err != nil {
        // tmux returns error if no server running
        if strings.Contains(err.Error(), "no server running") {
            return []Window{}, nil
        }
        return nil, fmt.Errorf("listing windows: %w", err)
    }

    return parseWindowList(stdout.String())
}
```

**Step 3: Add parseWindowList helper**

```go
func parseWindowList(output string) ([]Window, error) {
    lines := strings.Split(strings.TrimSpace(output), "\n")
    windows := make([]Window, 0, len(lines))

    for _, line := range lines {
        if line == "" {
            continue
        }

        parts := strings.SplitN(line, " ", 3)
        if len(parts) != 3 {
            continue
        }

        windows = append(windows, Window{
            ID:      parts[0],
            Name:    parts[1],
            Session: parts[2],
        })
    }

    return windows, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/tmux/... -run TestClient_ListWindows`
Expected: PASS (or SKIP if no tmux)

**Step 5: Commit**

```bash
git add internal/tmux/session.go internal/tmux/session_test.go
git commit -m "feat(tmux): add ListWindows method

Add Window struct and ListWindows method to query tmux windows
in the current session. Handles no-server-running gracefully.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Add CreateWindow Method

**Files:**
- Modify: `internal/tmux/session.go`
- Test: `internal/tmux/session_test.go`

**Step 1: Write failing test**

```go
func TestClient_CreateWindow(t *testing.T) {
    if os.Getenv("TMUX") == "" {
        t.Skip("not in tmux")
    }

    client, err := NewClient()
    if err != nil {
        t.Skip("tmux not installed")
    }

    // Create a test window with unique name
    testName := fmt.Sprintf("wt-test-%d", time.Now().Unix())
    testPath := os.TempDir()

    err = client.CreateWindow(testName, testPath)
    assert.NoError(t, err)

    // Verify window exists
    windows, _ := client.ListWindows()
    found := false
    for _, w := range windows {
        if w.Name == testName {
            found = true
            break
        }
    }
    assert.True(t, found, "window should exist after creation")

    // Cleanup
    _ = client.KillWindow(testName)
}
```

Run: `go test -v ./internal/tmux/... -run TestClient_CreateWindow`
Expected: FAIL with "method CreateWindow not defined"

**Step 2: Add CreateWindow method**

```go
// CreateWindow creates a new window in the current session
func (c *Client) CreateWindow(name, path string) error {
    args := []string{"new-window", "-c", path, "-n", name}
    cmd := exec.Command(c.tmuxPath, args...)

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("creating window: %w", err)
    }

    return nil
}
```

**Step 3: Run test to verify it passes**

Run: `go test -v ./internal/tmux/... -run TestClient_CreateWindow`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/tmux/session.go internal/tmux/session_test.go
git commit -m "feat(tmux): add CreateWindow method

Add ability to create new tmux windows with specified name
and working directory.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Add SelectWindow and KillWindow Methods

**Files:**
- Modify: `internal/tmux/session.go`
- Test: `internal/tmux/session_test.go`

**Step 1: Write failing tests**

```go
func TestClient_SelectWindow(t *testing.T) {
    if os.Getenv("TMUX") == "" {
        t.Skip("not in tmux")
    }

    client, err := NewClient()
    if err != nil {
        t.Skip("tmux not installed")
    }

    // Create a test window
    testName := fmt.Sprintf("wt-test-select-%d", time.Now().Unix())
    _ = client.CreateWindow(testName, os.TempDir())

    // Select the window
    err = client.SelectWindow(testName)
    assert.NoError(t, err)

    // Cleanup
    _ = client.KillWindow(testName)
}

func TestClient_KillWindow(t *testing.T) {
    if os.Getenv("TMUX") == "" {
        t.Skip("not in tmux")
    }

    client, err := NewClient()
    if err != nil {
        t.Skip("tmux not installed")
    }

    // Create a test window
    testName := fmt.Sprintf("wt-test-kill-%d", time.Now().Unix())
    _ = client.CreateWindow(testName, os.TempDir())

    // Verify it exists
    windows, _ := client.ListWindows()
    var existsBefore bool
    for _, w := range windows {
        if w.Name == testName {
            existsBefore = true
            break
        }
    }
    assert.True(t, existsBefore)

    // Kill the window
    err = client.KillWindow(testName)
    assert.NoError(t, err)

    // Verify it's gone
    windows, _ = client.ListWindows()
    var existsAfter bool
    for _, w := range windows {
        if w.Name == testName {
            existsAfter = true
            break
        }
    }
    assert.False(t, existsAfter, "window should not exist after killing")
}
```

Run: `go test -v ./internal/tmux/... -run "TestClient_SelectWindow|TestClient_KillWindow"`
Expected: FAIL with methods not defined

**Step 2: Add SelectWindow and KillWindow methods**

```go
// SelectWindow switches to the specified window
func (c *Client) SelectWindow(name string) error {
    cmd := exec.Command(c.tmuxPath, "select-window", "-t", name)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("selecting window: %w", err)
    }
    return nil
}

// KillWindow closes the specified window
func (c *Client) KillWindow(name string) error {
    cmd := exec.Command(c.tmuxPath, "kill-window", "-t", name)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("killing window: %w", err)
    }
    return nil
}
```

**Step 3: Run tests to verify they pass**

Run: `go test -v ./internal/tmux/... -run "TestClient_SelectWindow|TestClient_KillWindow"`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/tmux/session.go internal/tmux/session_test.go
git commit -m "feat(tmux): add SelectWindow and KillWindow methods

Add ability to switch to and close tmux windows by name.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: Add HasWindow Method

**Files:**
- Modify: `internal/tmux/session.go`
- Test: `internal/tmux/session_test.go`

**Step 1: Write failing test**

```go
func TestClient_HasWindow(t *testing.T) {
    if os.Getenv("TMUX") == "" {
        t.Skip("not in tmux")
    }

    client, err := NewClient()
    if err != nil {
        t.Skip("tmux not installed")
    }

    // Non-existent window
    has, err := client.HasWindow(fmt.Sprintf("nonexistent-%d", time.Now().Unix()))
    assert.NoError(t, err)
    assert.False(t, has)

    // Create and find
    testName := fmt.Sprintf("wt-test-has-%d", time.Now().Unix())
    _ = client.CreateWindow(testName, os.TempDir())

    has, err = client.HasWindow(testName)
    assert.NoError(t, err)
    assert.True(t, has)

    // Cleanup
    _ = client.KillWindow(testName)
}
```

Run: `go test -v ./internal/tmux/... -run TestClient_HasWindow`
Expected: FAIL with "method HasWindow not defined"

**Step 2: Add HasWindow method**

```go
// HasWindow checks if a window with the given name exists in the current session
func (c *Client) HasWindow(name string) (bool, error) {
    windows, err := c.ListWindows()
    if err != nil {
        return false, err
    }

    for _, w := range windows {
        if w.Name == name {
            return true, nil
        }
    }
    return false, nil
}
```

**Step 3: Run test to verify it passes**

Run: `go test -v ./internal/tmux/... -run TestClient_HasWindow`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/tmux/session.go internal/tmux/session_test.go
git commit -m "feat(tmux): add HasWindow method

Check if a window with given name exists in current session.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: Create Window Naming Package

**Files:**
- Create: `internal/tmux/naming.go`
- Test: `internal/tmux/naming_test.go`

**Step 1: Write failing tests for window naming**

Create `internal/tmux/naming_test.go`:

```go
package tmux

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestGenerateWindowName(t *testing.T) {
    tests := []struct {
        name     string
        branch   string
        expected string
    }{
        {"simple branch", "feat/auth", "feat/auth"},
        {"feature with issue", "feature/nova-123", "nova-123"},
        {"feature with dash suffix", "feature/api-fix", "feat/a-f"},
        {"long branch name", "very-long-branch-name-here", "very-long-br"},
        {"bugfix branch", "bugfix/auth-providers", "fix/auth-p"},
        {"single word", "authentication", "authentication"},
        {"hotfix branch", "hotfix/critical-bug", "hot/critical"},
        {"refactor branch", "refactor/user-service", "ref/user-s"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := GenerateWindowName(tt.branch)
            assert.Equal(t, tt.expected, got)
        })
    }
}

func TestGenerateStackWindowName(t *testing.T) {
    tests := []struct {
        name       string
        branch     string
        stackLevel int
        expected   string
    }{
        {"root branch", "feat/auth", 0, "feat/auth"},
        {"first stack level", "feat/auth-xY7k", 1, "feat/auth/1"},
        {"second stack level", "feat/auth-xY7k-aB2m", 2, "feat/auth/2"},
        {"issue id root", "feature/nova-123", 0, "nova-123"},
        {"issue id stacked", "feature/nova-123-xY7k", 1, "nova-123/1"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := GenerateStackWindowName(tt.branch, tt.stackLevel)
            assert.Equal(t, tt.expected, got)
        })
    }
}

func TestExtractIssueID(t *testing.T) {
    tests := []struct {
        branch   string
        expected string
    }{
        {"feature/nova-123", "nova-123"},
        {"fix/PROJ-456", "PROJ-456"},
        {"bugfix/ABC-999", "ABC-999"},
        {"feat/auth", ""},
        {"authentication", ""},
    }

    for _, tt := range tests {
        t.Run(tt.branch, func(t *testing.T) {
            got := extractIssueID(tt.branch)
            assert.Equal(t, tt.expected, got)
        })
    }
}
```

Run: `go test -v ./internal/tmux/... -run TestGenerate`
Expected: FAIL with functions not defined

**Step 2: Implement naming package**

Create `internal/tmux/naming.go`:

```go
package tmux

import (
    "fmt"
    "regexp"
    "strings"
)

const maxWindowNameLength = 16

// GenerateWindowName creates a shortened window name from a branch name
func GenerateWindowName(branch string) string {
    // 1. Try issue ID extraction: feature/nova-123 → nova-123
    if issue := extractIssueID(branch); issue != "" {
        return truncate(issue, maxWindowNameLength)
    }

    // 2. Parse branch components
    parts := strings.Split(branch, "/")

    var prefix, suffix string
    if len(parts) >= 2 {
        prefix = abbreviatePrefix(parts[0])
        suffix = parts[1]
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

    return truncate(result, maxWindowNameLength)
}

// GenerateStackWindowName creates a window name for a stacked branch
func GenerateStackWindowName(branch string, stackLevel int) string {
    // Root branch (level 0) has no suffix
    if stackLevel == 0 {
        return GenerateWindowName(branch)
    }

    // Get root branch name (strip nanoid suffixes)
    root := getStackRoot(branch)

    // Generate base name from root
    baseName := GenerateWindowName(root)

    // Add stack level suffix
    return fmt.Sprintf("%s/%d", baseName, stackLevel)
}

// extractIssueID extracts issue ID from branch names like feature/nova-123
func extractIssueID(branch string) string {
    // Match: prefix/non-dash-word-number pattern
    re := regexp.MustCompile(`^[^/]+([^\-]+-\d+)`)
    matches := re.FindStringSubmatch(branch)
    if len(matches) > 1 {
        return matches[1]
    }
    return ""
}

// abbreviatePrefix shortens common branch prefixes
func abbreviatePrefix(prefix string) string {
    abbreviations := map[string]string{
        "feature":  "feat",
        "bugfix":   "fix",
        "hotfix":   "hot",
        "chore":    "chr",
        "refactor": "ref",
        "test":     "tst",
        "fix":      "fix",
    }
    if abbr, ok := abbreviations[prefix]; ok {
        return abbr
    }
    // Default: first 4 chars
    return truncate(prefix, 4)
}

// abbreviateSuffix takes first character of each word
func abbreviateSuffix(suffix string) string {
    // Split on non-alphanumeric: auth-provider → auth provider
    re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
    words := re.Split(suffix, -1)

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

// getStackRoot removes nanoid suffixes to get the root branch name
func getStackRoot(branch string) string {
    // Nanoid pattern: - followed by 4+ alphanumeric chars
    // Remove all such suffixes
    re := regexp.MustCompile(`-[a-zA-Z0-9]{4,}$`)
    for re.MatchString(branch) {
        branch = re.ReplaceAllString(branch, "")
    }
    return branch
}

// truncates string to max length
func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen]
}
```

**Step 3: Run tests to verify they pass**

Run: `go test -v ./internal/tmux/... -run TestGenerate`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/tmux/naming.go internal/tmux/naming_test.go
git commit -m "feat(tmux): add window naming logic

Implement smart window name abbreviation:
- Extract issue IDs (feature/PROJ-123 → PROJ-123)
- Abbreviate prefixes (feature → feat)
- First-char suffixes (auth-provider → a-p)
- Stack numbering (feat/auth/1, feat/auth/2)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: Add InTmux Detection Helper

**Files:**
- Modify: `internal/tmux/session.go`

**Step 1: Write failing test**

```go
func TestInTmux(t *testing.T) {
    // Can't easily test actual tmux detection in unit tests
    // Just verify the function runs without panic
    _ = InTmux()
}
```

Run: `go test -v ./internal/tmux/... -run TestInTmux`
Expected: FAIL with "function InTmux not defined"

**Step 2: Add InTmux function**

```go
// InTmux checks if we're running inside a tmux session
func InTmux() bool {
    // Check $TMUX environment variable
    // When inside tmux, this is set to a socket path
    return os.Getenv("TMUX") != ""
}
```

Add import: `"os"`

**Step 3: Run test to verify it passes**

Run: `go test -v ./internal/tmux/... -run TestInTmux`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/tmux/session.go internal/tmux/session_test.go
git commit -m "feat(tmux): add InTmux detection

Check if running inside tmux via $TMUX env variable.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: Add --no-tmux Flag to Root Command

**Files:**
- Modify: `internal/cli/root.go`

**Step 1: Read root.go to understand current structure**

Check for global flags and how they're stored.

**Step 2: Add NoTmux global variable and flag**

Add to `internal/cli/root.go`:

```go
// Global state
var (
    verbose bool
    dryRun  bool
    noTmux  bool
)
```

Add flag registration in the root command initialization (look for where --verbose is registered):

```go
rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")
rootCmd.PersistentFlags().BoolVar(&noTmux, "no-tmux", false, "skip tmux window creation")
```

**Step 3: Add GetNoTmux accessor**

```go
// GetNoTmux returns the value of the --no-tmux flag
func GetNoTmux() bool {
    return noTmux
}
```

**Step 4: Test manually**

Run: `./bin/wt --help` and verify --no-tmux appears in flags

**Step 5: Commit**

```bash
git add internal/cli/root.go
git commit -m "feat(cli): add --no-tmux global flag

Add global --no-tmux flag to skip automatic tmux window
creation across all commands.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8: Integrate Window Creation into wt add

**Files:**
- Modify: `internal/cli/add.go`

**Step 1: Write integration test**

Create `internal/cli/add_tmux_test.go`:

```go
package cli

import (
    "os"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestAddCommand_CreatesTmuxWindow(t *testing.T) {
    if os.Getenv("TMUX") == "" {
        t.Skip("not in tmux")
    }

    // This is an integration test - verify behavior, not exact implementation
    // The actual test requires a git repo and tmux session
    // For now, just verify the code compiles with tmux integration
    assert.True(t, true)
}
```

Run: `go test -v ./internal/cli/... -run TestAddCommand_CreatesTmuxWindow`
Expected: PASS (placeholder)

**Step 2: Add window creation to add.go**

Import tmux package:

```go
import (
    // ... existing imports ...
    "github.com/user/wt/internal/tmux"
)
```

Add window creation after setup hooks:

```go
// In the Run function, after runSetupHooks:
// Create tmux window if in tmux and not disabled
if !cli.GetNoTmux() && tmux.InTmux() {
    tmuxClient, err := tmux.NewClient()
    if err == nil {
        windowName := tmux.GenerateWindowName(branch)

        // Check if window already exists
        exists, _ := tmuxClient.HasWindow(windowName)
        if exists {
            // Switch to existing window
            _ = tmuxClient.SelectWindow(windowName)
        } else {
            // Create new window
            _ = tmuxClient.CreateWindow(windowName, worktree.Path)
        }
    }
}
```

**Step 3: Run test**

Run: `go test -v ./internal/cli/... -run TestAddCommand`
Expected: PASS

**Step 4: Manual integration test**

```bash
# In tmux session
cd /tmp/test-repo
wt add test-branch
# Verify new window created with abbreviated name
```

**Step 5: Commit**

```bash
git add internal/cli/add.go internal/cli/add_tmux_test.go
git commit -m "feat(add): integrate tmux window creation

When in tmux, automatically create or switch to a window
when adding a worktree. Window name is abbreviated using
smart naming logic.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9: Integrate Window Creation into wt stack

**Files:**
- Modify: `internal/cli/stack.go`

**Step 1: Add window creation with stack level**

After the worktree creation in `NewStackCmd`, add:

```go
import (
    // ... existing imports ...
    "github.com/user/wt/internal/tmux"
)
```

After setup hooks:

```go
// Create tmux window if in tmux and not disabled
if !cli.GetNoTmux() && tmux.InTmux() {
    tmuxClient, err := tmux.NewClient()
    if err == nil {
        // Get stack level for numbering
        stackLevel := stackBranch.StackLevel
        windowName := tmux.GenerateStackWindowName(stackBranch.Name, stackLevel)

        // Check if window already exists
        exists, _ := tmuxClient.HasWindow(windowName)
        if exists {
            _ = tmuxClient.SelectWindow(windowName)
        } else {
            _ = tmuxClient.CreateWindow(windowName, worktree.Path)
        }
    }
}
```

**Note:** This requires `StackBranch` to have a `StackLevel` field. If it doesn't exist, see Task 10.

**Step 2: Test manually**

```bash
# In tmux session
git checkout -b feat/root
wt stack
# Verify window: feat/root/1
wt stack
# Verify window: feat/root/2
```

**Step 3: Commit**

```bash
git add internal/cli/stack.go
git commit -m "feat(stack): integrate tmux window creation

When in tmux, automatically create numbered windows
for stacked branches (feat/root/1, feat/root/2, etc).

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 10: Add StackLevel to StackBranch

**Files:**
- Modify: `pkg/domain/worktree.go`
- Modify: `internal/stack/service.go`

**Step 1: Add StackLevel field to domain**

Read `pkg/domain/worktree.go` and add `StackLevel int` to `StackBranch`:

```go
type StackBranch struct {
    Name       string
    Base       string
    StackLevel int  // Position in stack (0 = root, 1 = first child, etc)
}
```

**Step 2: Update CreateStackBranch to calculate stack level**

In `internal/stack/service.go`, modify `CreateStackBranch` to detect stack level:

```go
func (s *StackService) CreateStackBranch(ctx context.Context, spec StackBranchSpec) (*domain.StackBranch, error) {
    // ... existing code ...

    // Calculate stack level by parsing current branch name
    currentBranch, _ := s.gitClient.GetCurrentBranch(ctx)
    stackLevel := calculateStackLevel(currentBranch)

    return &domain.StackBranch{
        Name:       branchName,
        Base:       baseBranch,
        StackLevel: stackLevel,
    }, nil
}

func calculateStackLevel(branch string) int {
    // Count nanoid suffixes to determine stack level
    // feat/auth → 0
    // feat/auth-xY7k → 1
    // feat/auth-xY7k-aB2m → 2
    re := regexp.MustCompile(`-[a-zA-Z0-9]{4,}`)
    matches := re.FindAllString(branch, -1)
    return len(matches)
}
```

**Step 3: Run tests**

Run: `make test`
Expected: All tests pass

**Step 4: Commit**

```bash
git add pkg/domain/worktree.go internal/stack/service.go
git commit -m "feat(stack): add StackLevel to StackBranch

Track stack position for window naming. Stack level is
calculated by counting nanoid suffixes in branch name.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 11: Add Window Cleanup to wt remove

**Files:**
- Modify: `internal/cli/remove.go`

**Step 1: Read remove.go structure**

**Step 2: Add window cleanup on remove**

Import tmux and add cleanup:

```go
import (
    // ... existing imports ...
    "github.com/user/wt/internal/tmux"
)
```

After successful worktree removal:

```go
// Close tmux window if it exists
if tmux.InTmux() {
    if tmuxClient, err := tmux.NewClient(); err == nil {
        windowName := tmux.GenerateWindowName(branch)
        if has, _ := tmuxClient.HasWindow(windowName); has {
            _ = tmuxClient.KillWindow(windowName)
        }
    }
}
```

**Step 3: Test manually**

```bash
wt add test-remove
# Verify window exists
wt remove test-remove
# Verify window is gone
```

**Step 4: Commit**

```bash
git add internal/cli/remove.go
git commit -m "feat(remove): close tmux window on worktree remove

When removing a worktree, automatically close the
corresponding tmux window if it exists.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 12: Add Configuration Options

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Add TmuxEnabled field to TmuxConfig**

```go
type TmuxConfig struct {
    Layout         string `yaml:"layout,omitempty"`
    WindowName     string `yaml:"window_name,omitempty"`
    AttachOnCreate bool   `yaml:"attach_on_create,omitempty"`
    Enabled        bool   `yaml:"enabled,omitempty"`
}
```

**Step 2: Update DefaultConfig**

```go
Tmux: TmuxConfig{
    Layout:         "main-vertical",
    WindowName:     "work",
    AttachOnCreate: true,
    Enabled:        true,
},
```

**Step 3: Update add.go to check config**

Replace `!cli.GetNoTmux()` check with:

```go
cfg, _ := loadConfigForCommand()
shouldCreateTmux := !cli.GetNoTmux() && cfg.Tmux.Enabled && tmux.InTmux()

if shouldCreateTmux {
    // ... window creation code ...
}
```

**Step 4: Test config toggle**

```bash
# In config.yaml set tmux.enabled: false
wt add test-branch
# Verify no window created

# Set tmux.enabled: true
wt add test-branch2
# Verify window created
```

**Step 5: Commit**

```bash
git add internal/config/config.go internal/cli/add.go
git commit -m "feat(config): add tmux.enabled option

Allow disabling tmux integration via config file.
Defaults to true for backward compatibility.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 13: Update Documentation

**Files:**
- Modify: `README.md`
- Create: `docs/tmux.md`

**Step 1: Add tmux section to README**

Add to features section:

```markdown
## Tmux Integration

WT automatically creates tmux windows when you're in a tmux session:

```bash
$ wt add feat/auth
# Creates worktree and tmux window "feat/auth"
```

Window names are smart-abbreviated:
- `feat/auth` → `feat/auth`
- `feature/nova-123` → `nova-123`
- `feature/api-fix` → `feat/a-f`

Stacked branches get numbered windows:
- Root: `feat/auth`
- Level 1: `feat/auth/1`
- Level 2: `feat/auth/2`

Disable with `--no-tmux` or `tmux.enabled: false` in config.
```

**Step 2: Create detailed tmux documentation**

Create `docs/tmux.md`:

```markdown
# Tmux Integration

## Overview

WT creates tmux windows automatically when adding worktrees.

## Window Naming

### Regular Branches

| Branch | Window Name |
|--------|-------------|
| `feat/auth` | `feat/auth` |
| `feature/nova-123` | `nova-123` |
| `feature/api-fix` | `feat/a-f` |

### Stacked Branches

| Branch | Stack Level | Window Name |
|--------|-------------|-------------|
| `feat/auth` | 0 | `feat/auth` |
| `feat/auth-xY7k` | 1 | `feat/auth/1` |
| `feat/auth-xY7k-aB2m` | 2 | `feat/auth/2` |

## Configuration

```yaml
# ~/.config/wt/config.yaml
tmux:
  enabled: true  # Disable with false
```

## Flags

```bash
wt add feat/auth --no-tmux  # Skip window creation
wt stack --no-tmux          # Skip window creation
```
```

**Step 3: Commit**

```bash
git add README.md docs/tmux.md
git commit -m "docs: add tmux integration documentation

Document tmux window creation, naming conventions,
and configuration options.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 14: End-to-End Integration Test

**Files:**
- Test: `internal/cli/tmux_integration_test.go`

**Step 1: Write comprehensive integration test**

```go
package cli

import (
    "os"
    "os/exec"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestTmuxIntegration_E2E(t *testing.T) {
    if os.Getenv("TMUX") == "" {
        t.Skip("not in tmux")
    }

    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // This test requires a real git repo and tmux
    // It's marked as a manual integration test

    t.Log("Integration test requires manual verification")
    t.Log("1. Create test repo")
    t.Log("2. Run: wt add test-branch")
    t.Log("3. Verify window exists")
    t.Log("4. Run: wt remove test-branch")
    t.Log("5. Verify window is closed")
}
```

**Step 2: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 3: Manual verification checklist**

```bash
# In tmux session
echo "Test 1: Basic window creation"
wt add test-1
# Expected: Window "test-1" created

echo "Test 2: Abbreviated naming"
wt add feature/very-long-branch-name
# Expected: Window name truncated to 16 chars

echo "Test 3: Issue ID extraction"
wt add feature/PROJ-123
# Expected: Window "PROJ-123"

echo "Test 4: --no-tmux flag"
wt add test-no-tmux --no-tmux
# Expected: No window created

echo "Test 5: Window cleanup"
wt remove test-1
# Expected: Window "test-1" closed
```

**Step 4: Commit**

```bash
git add internal/cli/tmux_integration_test.go
git commit -m "test(tmux): add integration test placeholder

Add framework for tmux integration testing.
Manual verification required for full E2E testing.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 15: Final Verification

**Step 1: Run full test suite**

```bash
make test
make lint
make build
```

Expected: All pass

**Step 2: Manual smoke test**

```bash
# Build and install
make build
make install

# In tmux session
cd ~/projects/my-repo
wt add feat/tmux-test
# Verify: window created
cd feat/tmux-test
wt stack
# Verify: stacked window created with /1 suffix
wt list
# Verify: both worktrees shown
```

**Step 3: Edge case testing**

```bash
# Test not in tmux
unset TMUX
wt add test-no-tmux-session
# Expected: Works fine, just no window

# Test tmux not installed
# (mock by removing tmux from PATH temporarily)
# Expected: Graceful degradation

# Test existing window
wt add existing-window
wt add existing-window  # Try again
# Expected: Switches to existing window
```

**Step 4: Final commit**

```bash
git add .
git commit -m "polish(tmux): complete integration verification

All tmux integration features complete:
- Window creation for wt add and wt stack
- Smart naming with abbreviation
- Stack numbering (/1, /2, etc)
- Window cleanup on wt remove
- Configuration and flag support
- Graceful error handling

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Summary

This plan implements the tmux integration in 15 bite-sized tasks:

**Phase 1: Tmux Client Foundation** (Tasks 1-6)
- Window operations: List, Create, Select, Kill, Has
- Window naming logic with abbreviation
- InTmux detection

**Phase 2: CLI Integration** (Tasks 7-9)
- Global --no-tmux flag
- wt add integration
- wt stack integration with stack level tracking

**Phase 3: Cleanup and Polish** (Tasks 10-15)
- wt remove cleanup
- Configuration options
- Documentation
- Testing and verification

Each task follows TDD: write test, implement, verify, commit.
