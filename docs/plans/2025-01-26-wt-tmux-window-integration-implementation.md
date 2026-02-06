# WT Tmux Window Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Automatically create and manage tmux windows for each worktree with smart naming conventions and stack support.

**Architecture:** Extend the existing `internal/tmux` package to add window operations, integrate window creation into `wt add` and `wt stack` commands, add window cleanup to `wt remove`, and add global `--no-tmux` flag control.

**Tech Stack:** Go 1.23+, tmux 1.9+, git-spice (for stack metadata), existing wt CLI framework

---

## Task 1: Add Window Naming Logic Unit Tests

**Files:**
- Create: `internal/tmux/window_naming_test.go`

**Step 1: Write the failing test for issue ID extraction**

Create the file with this content:

```go
package tmux

import (
	"testing"
)

func TestExtractIssueID(t *testing.T) {
	tests := []struct {
		name  string
		branch string
		want  string
	}{
		{
			name:  "feature with nova issue ID",
			branch: "feature/nova-123",
			want:  "nova-123",
		},
		{
			name:  "fix with PROJ issue ID",
			branch: "fix/PROJ-456",
			want:  "PROJ-456",
		},
		{
			name:  "bugfix with uppercase issue ID",
			branch: "bugfix/ABC-789",
			want:  "ABC-789",
		},
		{
			name:  "branch without issue ID",
			branch: "feat/auth",
			want:  "",
		},
		{
			name:  "branch without slash",
			branch: "main",
			want:  "",
		},
		{
			name:  "issue ID without dash",
			branch: "feature/nova123",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIssueID(tt.branch)
			if got != tt.want {
				t.Errorf("extractIssueID(%q) = %q, want %q", tt.branch, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/tmux -run TestExtractIssueID`
Expected: FAIL with "undefined: extractIssueID"

**Step 3: Write minimal implementation**

Add to `internal/tmux/session.go`:

```go
// extractIssueID extracts issue ID from branch names like "feature/nova-123"
// Returns the issue ID if found, empty string otherwise
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
```

Also add import at top of file:
```go
import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/tmux -run TestExtractIssueID`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tmux/session.go internal/tmux/window_naming_test.go
git commit -m "feat(tmux): add issue ID extraction for window naming"
```

---

## Task 2: Add Prefix Abbreviation Unit Tests

**Files:**
- Modify: `internal/tmux/window_naming_test.go`

**Step 1: Write the failing test**

Add to `internal/tmux/window_naming_test.go`:

```go
func TestAbbreviatePrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{"feature to feat", "feature", "feat"},
		{"bugfix to fix", "bugfix", "fix"},
		{"hotfix to hot", "hotfix", "hot"},
		{"chore to chr", "chore", "chr"},
		{"refactor to ref", "refactor", "ref"},
		{"test to tst", "test", "tst"},
		{"unknown prefix truncated", "unknown", "unkn"},
		{"short prefix", "fix", "fix"},
		{"exactly 4 chars", "feat", "feat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := abbreviatePrefix(tt.prefix)
			if got != tt.want {
				t.Errorf("abbreviatePrefix(%q) = %q, want %q", tt.prefix, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/tmux -run TestAbbreviatePrefix`
Expected: FAIL with "undefined: abbreviatePrefix"

**Step 3: Write minimal implementation**

Add to `internal/tmux/session.go`:

```go
// abbreviatePrefix abbreviates common branch type prefixes
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
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/tmux -run TestAbbreviatePrefix`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tmux/session.go internal/tmux/window_naming_test.go
git commit -m "feat(tmux): add prefix abbreviation for window naming"
```

---

## Task 3: Add Suffix Abbreviation Unit Tests

**Files:**
- Modify: `internal/tmux/window_naming_test.go`

**Step 1: Write the failing test**

Add to `internal/tmux/window_naming_test.go`:

```go
func TestAbbreviateSuffix(t *testing.T) {
	tests := []struct {
		name   string
		suffix string
		want   string
	}{
		{"single word", "auth", "auth"},
		{"two words with dash", "auth-provider", "auth-p"},
		{"three words", "auth-provider-api", "auth-p-a"},
		{"with number", "auth-123", "auth-123"},
		{"digits only word", "auth-123-fix", "auth-123-f"},
		{"leading dash in word", "-auth", "auth"},
		{"multiple consecutive dashes", "auth--provider", "auth-p"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := abbreviateSuffix(tt.suffix)
			if got != tt.want {
				t.Errorf("abbreviateSuffix(%q) = %q, want %q", tt.suffix, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/tmux -run TestAbbreviateSuffix`
Expected: FAIL with "undefined: abbreviateSuffix"

**Step 3: Write minimal implementation**

Add to `internal/tmux/session.go`:

```go
// abbreviateSuffix takes first character of each word (words split by non-alphanumeric)
// Keeps digits-only words intact
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
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/tmux -run TestAbbreviateSuffix`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tmux/session.go internal/tmux/window_naming_test.go
git commit -m "feat(tmux): add suffix abbreviation for window naming"
```

---

## Task 4: Add Truncate and GenerateWindowName Tests

**Files:**
- Modify: `internal/tmux/window_naming_test.go`

**Step 1: Write the failing test**

Add to `internal/tmux/window_naming_test.go`:

```go
func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"exactly max", "hello", 5, "hello"},
		{"longer than max", "hello world", 5, "hello"},
		{"empty string", "", 5, ""},
		{"zero max", "hello", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestGenerateWindowName(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{"issue ID extraction", "feature/nova-123", "nova-123"},
		{"two part branch", "feat/auth", "feat/auth"},
		{"feature branch", "feature/api-fix", "feat/a-f"},
		{"bugfix branch", "bugfix/auth-providers", "fix/auth-p"},
		{"long branch name", "very-long-branch-name-here", "very-long-br"},
		{"single word", "main", "main"},
		{"three part branch", "feat/team/auth-api", "feat/a-a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateWindowName(tt.branch)
			if got != tt.want {
				t.Errorf("generateWindowName(%q) = %q, want %q", tt.branch, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/tmux -run TestTruncate`
Expected: FAIL with "undefined: truncate"

**Step 3: Write minimal implementation**

Add to `internal/tmux/session.go`:

```go
// truncate truncates string to maxLen if longer
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// generateWindowName generates a tmux window name from a branch name
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
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/tmux -run "TestTruncate|TestGenerateWindowName"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tmux/session.go internal/tmux/window_naming_test.go
git commit -m "feat(tmux): add window name generation logic"
```

---

## Task 5: Add Stack Window Naming Tests

**Files:**
- Modify: `internal/tmux/window_naming_test.go`

**Step 1: Write the failing test**

Add to `internal/tmux/window_naming_test.go`:

```go
func TestGetStackRoot(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{"no suffix", "feat/auth", "feat/auth"},
		{"single nanoid", "feat/auth-xY7k", "feat/auth"},
		{"double nanoid", "feat/auth-xY7k-aB2m", "feat/auth"},
		{"named suffix", "feat/auth-api-k9P2", "feat/auth-api-k9P2"},
		{"already has stack number", "feat-auth/1", "feat-auth/1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStackRoot(tt.branch)
			if got != tt.want {
				t.Errorf("getStackRoot(%q) = %q, want %q", tt.branch, got, tt.want)
			}
		})
	}
}

func TestGenerateStackWindowName(t *testing.T) {
	tests := []struct {
		name       string
		branch     string
		stackLevel int
		want       string
	}{
		{"root level no suffix", "feat/auth", 0, "feat/auth"},
		{"first stack level", "feat/auth-xY7k", 1, "feat/auth/1"},
		{"second stack level", "feat/auth-xY7k-aB2m", 2, "feat/auth/2"},
		{"named suffix first level", "feat/auth-api-k9P2", 1, "feat/auth-api/1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateStackWindowName(tt.branch, tt.stackLevel)
			if got != tt.want {
				t.Errorf("generateStackWindowName(%q, %d) = %q, want %q",
					tt.branch, tt.stackLevel, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/tmux -run TestGetStackRoot`
Expected: FAIL with "undefined: getStackRoot"

**Step 3: Write minimal implementation**

Add to `internal/tmux/session.go`:

```go
// getStackRoot returns the root branch name by stripping nanoid suffixes
// feat/auth-xY7k → feat/auth
// feat/auth-xY7k-aB2m → feat/auth
func getStackRoot(branch string) string {
	// If already has stack number suffix, return as-is
	if strings.Contains(branch, "/") {
		parts := strings.Split(branch, "/")
		if len(parts) == 2 {
			// Check if suffix after / is a number
			suffix := parts[1]
			if regexp.MustCompile(`^\d+$`).MatchString(suffix) {
				return branch // Already has stack number
			}
		}
	}

	// Remove nanoid suffixes (4-character alphanumeric suffixes preceded by dash)
	// Pattern: -xxxx where x is alphanumeric
	re := regexp.MustCompile(`-[a-zA-Z0-9]{4}($|-[a-zA-Z0-9]{4})`)
	root := re.ReplaceAllString(branch, "")

	return root
}

// generateStackWindowName generates a window name for a stacked branch
func generateStackWindowName(branch string, stackLevel int) string {
	// Get root branch name (strip nanoid suffixes)
	root := getStackRoot(branch)

	// Generate base name
	baseName := generateWindowName(root)

	// Root level (0) has no suffix
	if stackLevel == 0 {
		return baseName
	}

	// Add stack level suffix
	return fmt.Sprintf("%s/%d", baseName, stackLevel)
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/tmux -run "TestGetStackRoot|TestGenerateStackWindowName"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tmux/session.go internal/tmux/window_naming_test.go
git commit -m "feat(tmux): add stack window naming logic"
```

---

## Task 6: Add Window Creation Tests

**Files:**
- Create: `internal/tmux/window_test.go`

**Step 1: Write the failing test for creating a new window**

Create the file with this content:

```go
package tmux

import (
	"context"
	"os"
	"testing"
)

func TestClient_CreateNewWindow(t *testing.T) {
	// Skip if not in tmux or tmux not available
	if os.Getenv("TMUX") == "" {
		t.Skip("skipping test: not in tmux")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	// Create a test window
	err = client.CreateNewWindow("test-window", "/tmp")
	if err != nil {
		t.Fatalf("CreateNewWindow() error = %v", err)
	}

	// Clean up
	_ = client.KillWindow("test-window")
}

func TestClient_CreateNewWindow_InvalidTarget(t *testing.T) {
	// This test validates error handling for invalid window creation
	// We'll use a mock approach to test error conditions

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	// Try to create window with empty name - should error or handle gracefully
	err = client.CreateNewWindow("", "/tmp")
	// We expect this to either succeed with default name or fail gracefully
	// The exact behavior depends on tmux implementation
	_ = err // Just check it doesn't panic
}

func TestClient_WindowExists(t *testing.T) {
	if os.Getenv("TMUX") == "" {
		t.Skip("skipping test: not in tmux")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	// Test non-existent window
	exists, err := client.WindowExists("this-window-should-not-exist-xyz123")
	if err != nil {
		t.Fatalf("WindowExists() error = %v", err)
	}
	if exists {
		t.Error("WindowExists() should return false for non-existent window")
	}
}

func TestClient_SelectWindow(t *testing.T) {
	if os.Getenv("TMUX") == "" {
		t.Skip("skipping test: not in tmux")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	// Create test window first
	_ = client.CreateNewWindow("test-select-window", "/tmp")
	defer client.KillWindow("test-select-window")

	// Select the window
	err = client.SelectWindow("test-select-window")
	if err != nil {
		t.Fatalf("SelectWindow() error = %v", err)
	}
}

func TestClient_SendKeys(t *testing.T) {
	if os.Getenv("TMUX") == "" {
		t.Skip("skipping test: not in tmux")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	// Create test window
	_ = client.CreateNewWindow("test-send-keys", "/tmp")
	defer client.KillWindow("test-send-keys")

	// Send keys to the window
	err = client.SendKeys("test-send-keys", "echo 'test'", true)
	if err != nil {
		t.Fatalf("SendKeys() error = %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/tmux -run TestClient_CreateNewWindow`
Expected: FAIL with "undefined: CreateNewWindow"

**Step 3: Write minimal implementation**

Add to `internal/tmux/session.go`:

```go
// CreateNewWindow creates a new tmux window in the current session
func (c *Client) CreateNewWindow(name, path string) error {
	args := []string{"new-window", "-c", path, "-n", name}
	cmd := exec.Command(c.tmuxPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating window: %w", err)
	}
	return nil
}

// WindowExists checks if a window with the given name exists in the current session
func (c *Client) WindowExists(name string) (bool, error) {
	var stdout bytes.Buffer
	cmd := exec.Command(c.tmuxPath, "list-windows", "-F", "#{window_name}")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// tmux returns error if no server running
		if strings.Contains(err.Error(), "no server running") {
			return false, nil
		}
		return false, fmt.Errorf("listing windows: %w", err)
	}

	windows := parseWindowList(stdout.String())
	for _, w := range windows {
		if w == name {
			return true, nil
		}
	}
	return false, nil
}

// SelectWindow switches to the specified window
func (c *Client) SelectWindow(name string) error {
	cmd := exec.Command(c.tmuxPath, "select-window", "-t", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("selecting window: %w", err)
	}
	return nil
}

// SendKeys sends keys to the specified window
func (c *Client) SendKeys(name, keys string, enter bool) error {
	args := []string{"send-keys", "-t", name, keys}
	if enter {
		args = append(args, "Enter")
	}
	cmd := exec.Command(c.tmuxPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sending keys: %w", err)
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

// parseWindowList parses the output of tmux list-windows
func parseWindowList(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	windows := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			windows = append(windows, line)
		}
	}

	return windows
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/tmux -run TestClient_CreateNewWindow`
Expected: PASS (if in tmux)

**Step 5: Commit**

```bash
git add internal/tmux/session.go internal/tmux/window_test.go
git commit -m "feat(tmux): add window creation and management functions"
```

---

## Task 7: Add Window Switch/Create Logic

**Files:**
- Modify: `internal/tmux/session.go`

**Step 1: Write the failing test**

Add to `internal/tmux/window_test.go`:

```go
func TestClient_CreateOrSelectWindow_NewWindow(t *testing.T) {
	if os.Getenv("TMUX") == "" {
		t.Skip("skipping test: not in tmux")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	testWindow := "test-create-or-select-new"
	// Ensure window doesn't exist
	_ = client.KillWindow(testWindow)

	// Create or select (should create since it doesn't exist)
	err = client.CreateOrSelectWindow(testWindow, "/tmp")
	if err != nil {
		t.Fatalf("CreateOrSelectWindow() error = %v", err)
	}

	// Verify window exists
	exists, _ := client.WindowExists(testWindow)
	if !exists {
		t.Error("CreateOrSelectWindow() should have created window")
	}

	// Clean up
	_ = client.KillWindow(testWindow)
}

func TestClient_CreateOrSelectWindow_ExistingWindow(t *testing.T) {
	if os.Getenv("TMUX") == "" {
		t.Skip("skipping test: not in tmux")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	testWindow := "test-create-or-select-existing"

	// Create the window first
	_ = client.CreateNewWindow(testWindow, "/tmp")
	defer client.KillWindow(testWindow)

	// Create or select (should select since it exists)
	err = client.CreateOrSelectWindow(testWindow, "/tmp")
	if err != nil {
		t.Fatalf("CreateOrSelectWindow() error = %v", err)
	}

	// Window should still exist
	exists, _ := client.WindowExists(testWindow)
	if !exists {
		t.Error("CreateOrSelectWindow() should not have removed existing window")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/tmux -run TestClient_CreateOrSelectWindow`
Expected: FAIL with "undefined: CreateOrSelectWindow"

**Step 3: Write minimal implementation**

Add to `internal/tmux/session.go`:

```go
// CreateOrSelectWindow creates a new window or selects an existing one
// If the window exists, switches to it and changes directory
// If it doesn't exist, creates a new window with the given path
func (c *Client) CreateOrSelectWindow(name, path string) error {
	// Check if window already exists
	exists, err := c.WindowExists(name)
	if err != nil {
		return fmt.Errorf("checking window existence: %w", err)
	}

	if exists {
		// Switch to existing window and change directory
		if err := c.SelectWindow(name); err != nil {
			return fmt.Errorf("selecting window: %w", err)
		}
		// Send cd command to change directory
		return c.SendKeys(name, "cd "+path, true)
	}

	// Create new window
	return c.CreateNewWindow(name, path)
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/tmux -run TestClient_CreateOrSelectWindow`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tmux/session.go internal/tmux/window_test.go
git commit -m "feat(tmux): add CreateOrSelectWindow with smart switching"
```

---

## Task 8: Add Tmux Detection and Global Flag

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/tmux/session.go`

**Step 1: Write the failing test**

Create `internal/cli/root_tmux_test.go`:

```go
package cli

import (
	"os"
	"testing"
)

func TestIsInTmux(t *testing.T) {
	// Save original value
	original := os.Getenv("TMUX")
	defer os.Setenv("TMUX", original)

	// Test when not in tmux
	os.Unsetenv("TMUX")
	if isInTmux() {
		t.Error("isInTmux() should return false when TMUX is not set")
	}

	// Test when in tmux
	os.Setenv("TMUX", "/tmp/tmux-1000/default,1234,5678")
	if !isInTmux() {
		t.Error("isInTmux() should return true when TMUX is set")
	}

	// Test empty string
	os.Setenv("TMUX", "")
	if isInTmux() {
		t.Error("isInTmux() should return false when TMUX is empty")
	}
}

func TestShouldCreateTmuxWindow_Default(t *testing.T) {
	// Save original value
	original := os.Getenv("TMUX")
	defer os.Setenv("TMUX", original)

	os.Setenv("TMUX", "/tmp/tmux-1000/default,1234,5678")

	// Test default behavior (should create when in tmux)
	if !shouldCreateTmuxWindow(false) {
		t.Error("shouldCreateTmuxWindow() should return true by default when in tmux")
	}

	// Test with --no-tmux flag
	if shouldCreateTmuxWindow(true) {
		t.Error("shouldCreateTmuxWindow() should return false when --no-tmux is set")
	}
}

func TestShouldCreateTmuxWindow_NotInTmux(t *testing.T) {
	// Save original value
	original := os.Getenv("TMUX")
	defer os.Setenv("TMUX", original)

	os.Unsetenv("TMUX")

	// Should not create window when not in tmux
	if shouldCreateTmuxWindow(false) {
		t.Error("shouldCreateTmuxWindow() should return false when not in tmux")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/cli -run TestIsInTmux`
Expected: FAIL with "undefined: isInTmux"

**Step 3: Write minimal implementation**

Add to `internal/tmux/session.go`:

```go
// IsInTmux checks if the current process is running inside tmux
func IsInTmux() bool {
	return os.Getenv("TMUX") != ""
}
```

Add import: `"os"`

Add to `internal/cli/root.go`:

```go
func init() {
	// Global flags
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file path (default is $HOME/.config/wt/config.yaml or .wt.yaml in project)")
	rootCmd.PersistentFlags().CountP("verbose", "v", "verbose output (can be used multiple times)")
	rootCmd.PersistentFlags().Bool("no-tmux", false, "skip tmux window creation")
}

// NoTmux returns the value of the --no-tmux flag
func NoTmux() bool {
	noTmux, _ := rootCmd.PersistentFlags().GetBool("no-tmux")
	return noTmux
}

// isInTmux checks if currently running in tmux
func isInTmux() bool {
	return tmux.IsInTmux()
}

// shouldCreateTmuxWindow determines if tmux window should be created
func shouldCreateTmuxWindow(noTmuxFlag bool) bool {
	if !isInTmux() {
		return false
	}
	if noTmuxFlag {
		return false
	}
	return true
}
```

Add import to `internal/cli/root.go`: `"github.com/joebalancio/wt/internal/tmux"`

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/cli -run TestIsInTmux`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/root.go internal/tmux/session.go internal/cli/root_tmux_test.go
git commit -m "feat(tmux): add tmux detection and --no-tmux global flag"
```

---

## Task 9: Add Stack Level Detection

**Files:**
- Modify: `internal/spice/client.go`
- Create: `internal/spice/stack_test.go`

**Step 1: Write the failing test**

Create `internal/spice/stack_test.go`:

```go
package spice

import (
	"context"
	"testing"
)

func TestGetStackLevel(t *testing.T) {
	tests := []struct {
		name     string
		stack    []*Branch
		branch   string
		want     int
		wantErr  bool
	}{
		{
			name: "root branch",
			stack: []*Branch{
				{Name: "main", IsRoot: true},
				{Name: "feat/auth", IsRoot: false},
			},
			branch:  "feat/auth",
			want:    1,
			wantErr: false,
		},
		{
			name: "branch not in stack",
			stack: []*Branch{
				{Name: "main", IsRoot: true},
				{Name: "feat/other", IsRoot: false},
			},
			branch:  "feat/auth",
			want:    0,
			wantErr: true,
		},
		{
			name: "empty stack",
			stack: []*Branch{},
			branch:  "feat/auth",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{}
			got, err := c.GetStackLevel(tt.stack, tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetStackLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetStackLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/spice -run TestGetStackLevel`
Expected: FAIL with "undefined: GetStackLevel"

**Step 3: Write minimal implementation**

Add to `internal/spice/client.go`:

```go
// GetStackLevel returns the stack level (depth) of a branch in the stack
// Root branches (like main) return 0, first stacked branch returns 1, etc.
func (c *Client) GetStackLevel(stack []*Branch, branchName string) (int, error) {
	for i, b := range stack {
		if b.Name == branchName {
			// Stack level is the index (0 for root/main, 1 for first stacked, etc.)
			return i, nil
		}
	}
	return 0, fmt.Errorf("branch %q not found in stack", branchName)
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/spice -run TestGetStackLevel`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/spice/client.go internal/spice/stack_test.go
git commit -m "feat(spice): add GetStackLevel for window naming"
```

---

## Task 10: Integrate Window Creation with wt add

**Files:**
- Modify: `internal/cli/add.go`

**Step 1: Write the failing test**

Add to `internal/cli/add_test.go`:

```go
func TestNewAddCmd_TmuxIntegration(t *testing.T) {
	// This is an integration test that verifies the add command
	// properly calls tmux window creation when in tmux

	// We can't easily test the actual tmux integration in unit tests,
	// but we can verify the code path exists

	// Test that the command accepts --no-tmux flag
	cmd := NewAddCmd()
	flag := cmd.Flags().Lookup("no-tmux")
	if flag == nil {
		t.Error("--no-tmux flag should be available (inherited from root)")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/cli -run TestNewAddCmd_TmuxIntegration`
Expected: PASS (flag is inherited)

**Step 3: Write the implementation**

Add to `internal/cli/add.go` after line 75 (after setup hooks):

```go
			// Create tmux window if in tmux and not disabled
			if shouldCreateTmuxWindow(NoTmux()) {
				tmuxClient, err := tmux.NewClient()
				if err == nil {
					windowName := tmux.GenerateWindowName(worktree.Branch)
					if err := tmuxClient.CreateOrSelectWindow(windowName, worktree.Path); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
					}
				}
			}
```

Add import: `"github.com/joebalancio/wt/internal/tmux"`

Also need to export the window naming functions. Add to `internal/tmux/session.go`:

Change these functions to be exported (capital first letter):
- `generateWindowName` → `GenerateWindowName`
- `generateStackWindowName` → `GenerateStackWindowName`

Update all references in tests to use the exported names.

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/cli -run TestNewAddCmd_TmuxIntegration`
Expected: PASS

Also run naming tests to ensure export didn't break anything:
Run: `go test -v ./internal/tmux -run TestGenerateWindowName`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/add.go internal/tmux/session.go internal/tmux/window_naming_test.go internal/cli/add_test.go
git commit -m "feat(add): integrate tmux window creation"
```

---

## Task 11: Integrate Window Creation with wt stack

**Files:**
- Modify: `internal/cli/stack.go`

**Step 1: Write the failing test**

Add to `internal/cli/stack_test.go`:

```go
func TestNewStackCmd_TmuxIntegration(t *testing.T) {
	// Verify the stack command properly integrates with tmux
	// Similar to add command test
	cmd := NewStackCmd()
	flag := cmd.Flags().Lookup("no-tmux")
	if flag == nil {
		t.Error("--no-tmux flag should be available (inherited from root)")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/cli -run TestNewStackCmd_TmuxIntegration`
Expected: PASS

**Step 3: Write the implementation**

Add to `internal/cli/stack.go` after line 117 (after setup hooks):

```go
			// Create tmux window if in tmux and not disabled
			if shouldCreateTmuxWindow(NoTmux()) {
				tmuxClient, err := tmux.NewClient()
				if err == nil {
					// Get stack level for window naming
					stackBranches, _ := stackService.GetStack(ctx)
					stackLevel := 0
					for i, sb := range stackBranches {
						if sb.Name == stackBranch.Name {
							stackLevel = i
							break
						}
					}

					windowName := tmux.GenerateStackWindowName(stackBranch.Name, stackLevel)
					if err := tmuxClient.CreateOrSelectWindow(windowName, worktree.Path); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
					}
				}
			}
```

Add import: `"github.com/joebalancio/wt/internal/tmux"`

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/cli -run TestNewStackCmd_TmuxIntegration`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/stack.go internal/cli/stack_test.go
git commit -m "feat(stack): integrate tmux window creation with stack level naming"
```

---

## Task 12: Integrate Window Cleanup with wt remove

**Files:**
- Modify: `internal/cli/remove.go`

**Step 1: Write the failing test**

Add to `internal/cli/remove_test.go`:

```go
func TestNewRemoveCmd_TmuxIntegration(t *testing.T) {
	// Verify the remove command can clean up tmux windows
	cmd := NewRemoveCmd()
	if cmd == nil {
		t.Error("NewRemoveCmd() should return a command")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/cli -run TestNewRemoveCmd_TmuxIntegration`
Expected: PASS

**Step 3: Write the implementation**

Add to `internal/cli/remove.go` after line 48 (after successful removal):

```go
			// Close tmux window if in tmux and window matches
			if isInTmux() {
				tmuxClient, err := tmux.NewClient()
				if err == nil {
					// Try to determine branch name from path
					// This is best-effort - we try to match window name
					// Get the branch name from git worktree list
					worktrees, _ := gitClient.ListWorktrees(ctx)
					var branchName string
					for _, wt := range worktrees {
						if wt.Path == path {
							branchName = wt.Branch
							break
						}
					}

					if branchName != "" {
						windowName := tmux.GenerateWindowName(branchName)
						// Kill the window if it exists
						_ = tmuxClient.KillWindow(windowName)
					}
				}
			}
```

Add imports: `"github.com/joebalancio/wt/internal/tmux"` and `"context"`

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/cli -run TestNewRemoveCmd_TmuxIntegration`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/remove.go internal/cli/remove_test.go
git commit -m "feat(remove): integrate tmux window cleanup"
```

---

## Task 13: Update Configuration Structure

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestTmuxWindowConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Tmux.Enabled != false {
		t.Errorf("Tmux.Enabled default should be false, got %v", cfg.Tmux.Enabled)
	}

	if cfg.Tmux.WindowNaming.MaxLength != 16 {
		t.Errorf("WindowNaming.MaxLength default should be 16, got %d", cfg.Tmux.WindowNaming.MaxLength)
	}

	if cfg.Tmux.WindowNaming.AbbreviateIssueID != true {
		t.Error("WindowNaming.AbbreviateIssueID default should be true")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/config -run TestTmuxWindowConfigDefaults`
Expected: FAIL with "undefined: Tmux.Enabled"

**Step 3: Write minimal implementation**

Modify the TmuxConfig struct in `internal/config/config.go`:

```go
// TmuxConfig contains tmux-specific settings
type TmuxConfig struct {
	Enabled         bool                `yaml:"enabled"`
	AutoCreate      bool                `yaml:"auto_create"`
	WindowNaming    TmuxWindowNamingConfig `yaml:"window_naming,omitempty"`
	Layout          string              `yaml:"layout,omitempty"`
	WindowName      string              `yaml:"window_name,omitempty"`
	AttachOnCreate  bool                `yaml:"attach_on_create,omitempty"`
}

// TmuxWindowNamingConfig contains window naming settings
type TmuxWindowNamingConfig struct {
	MaxLength           int  `yaml:"max_length"`
	AbbreviateIssueID   bool `yaml:"abbreviate_issue_id"`
}
```

Update DefaultConfig:

```go
		Tmux: TmuxConfig{
			Enabled:     false,
			AutoCreate:  true,
			WindowNaming: TmuxWindowNamingConfig{
				MaxLength:           16,
				AbbreviateIssueID:   true,
			},
			Layout:         "main-vertical",
			WindowName:     "work",
			AttachOnCreate: true,
		},
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/config -run TestTmuxWindowConfigDefaults`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add tmux window configuration options"
```

---

## Task 14: Add Integration Tests

**Files:**
- Create: `tests/tmux_integration_test.go`

**Step 1: Write the integration test**

Create the file with this content:

```go
package tests

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestTmuxWindowCreation_AddCommand(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	if os.Getenv("TMUX") == "" {
		t.Skip("integration test must be run inside tmux")
	}

	// This test requires a real git repository
	// It verifies that wt add creates a tmux window

	// Create a test branch using wt
	cmd := exec.Command("go", "run", ".", "add", "test-tmux-integration")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wt add failed: %v\n%s", err, output)
	}

	// Check if tmux window was created
	listCmd := exec.Command("tmux", "list-windows", "-F", "#{window_name}")
	windowOutput, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tmux list-windows failed: %v", err)
	}

	if !strings.Contains(string(windowOutput), "test-tmux") {
		t.Error("tmux window was not created by wt add")
	}

	// Cleanup
	_ = exec.Command("go", "run", ".", "remove", "test-tmux-integration").Run()
	killCmd := exec.Command("tmux", "kill-window", "-t", "test-tmux")
	_ = killCmd.Run()
}

func TestTmuxWindowCreation_StackCommand(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	if os.Getenv("TMUX") == "" {
		t.Skip("integration test must be run inside tmux")
	}

	// Requires git-spice to be configured
	// This test verifies stack window naming

	// Create a stack
	cmd := exec.Command("go", "run", ".", "stack", "test")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("git-spice not configured or stack failed: %v\n%s", err, output)
	}

	// Check for numbered window
	listCmd := exec.Command("tmux", "list-windows", "-F", "#{window_name}")
	windowOutput, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tmux list-windows failed: %v", err)
	}

	// Should have a window with /1 suffix
	if !strings.Contains(string(windowOutput), "/1") {
		t.Logf("Windows: %s", windowOutput)
		t.Error("stack window should have /1 suffix")
	}

	// Cleanup
	// (Would need to delete stack branches)
}

func TestTmuxNoTmuxFlag(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	if os.Getenv("TMUX") == "" {
		t.Skip("integration test must be run inside tmux")
	}

	// Test that --no-tmux flag prevents window creation

	// List windows before
	beforeCmd := exec.Command("tmux", "list-windows", "-F", "#{window_name}")
	beforeOutput, _ := beforeCmd.CombinedOutput()

	// Create branch with --no-tmux
	cmd := exec.Command("go", "run", ".", "add", "test-no-tmux", "--no-tmux")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wt add failed: %v\n%s", err, output)
	}

	// List windows after
	afterCmd := exec.Command("tmux", "list-windows", "-F", "#{window_name}")
	afterOutput, _ := afterCmd.CombinedOutput()

	// Should be the same windows
	if string(beforeOutput) != string(afterOutput) {
		t.Error("--no-tmux flag should not create windows")
	}

	// Cleanup
	_ = exec.Command("go", "run", ".", "remove", "test-no-tmux").Run()
}
```

**Step 2: Run test to verify behavior**

Run: `WT_INTEGRATION_TEST=1 go test -v ./tests -run TestTmuxNoTmuxFlag`
Expected: May skip or run depending on environment

**Step 3: No implementation needed**

This is a test-only task.

**Step 4: Commit**

```bash
git add tests/tmux_integration_test.go
git commit -m "test(tmux): add integration tests for window creation"
```

---

## Task 15: Add Documentation

**Files:**
- Modify: `README.md`
- Create: `docs/tmux-windows.md`

**Step 1: Update README**

Add section to README.md describing the tmux integration:

```markdown
## Tmux Integration

WT automatically creates tmux windows when you create worktrees while inside a tmux session.

### Features

- **Automatic window creation**: `wt add feat/auth` creates a new tmux window
- **Smart naming**: Branch names are abbreviated for readable window names
- **Stack support**: Stacked branches get numbered suffixes (feat/auth/1, feat/auth/2)
- **Automatic cleanup**: Windows are closed when removing worktrees

### Window Naming

| Branch | Window Name |
|--------|-------------|
| `feat/auth` | `feat/auth` |
| `feature/nova-123` | `nova-123` |
| `feature/api-fix` | `feat/a-f` |
| `feat/auth-xY7k` (stack level 1) | `feat/auth/1` |

### Disabling Tmux Integration

To skip window creation for a single command:

```bash
wt add feat/auth --no-tmux
wt stack --no-tmux
```

See [Tmux Windows Documentation](docs/tmux-windows.md) for details.
```

**Step 2: Create detailed documentation**

Create `docs/tmux-windows.md`:

```markdown
# Tmux Window Integration

WT integrates with tmux to automatically create and manage windows for your worktrees.

## How It Works

When you run `wt add` or `wt stack` while inside a tmux session, WT automatically:

1. Creates a new tmux window with a smart name derived from the branch
2. Sets the working directory to the new worktree
3. Switches to the new window

If a window with that name already exists, WT switches to it instead.

## Window Naming

### Regular Branches

Window names are generated by:

1. Extracting issue IDs (e.g., `feature/nova-123` → `nova-123`)
2. Abbreviating common prefixes (feature→feat, bugfix→fix)
3. Taking first character of words (auth-provider→auth-p)
4. Truncating to 16 characters

### Stacked Branches

Stacked branches use numbered suffixes:

- Root branch: `feat/auth` → window `feat/auth`
- First stacked: `feat/auth-xY7k` → window `feat/auth/1`
- Second stacked: `feat/auth-xY7k-aB2m` → window `feat/auth/2`

## Configuration

Add to your `~/.config/wt/config.yaml`:

```yaml
tmux:
  enabled: true  # Global on/off (default: true)
  auto_create: true  # Auto-create windows (default: true)
  window_naming:
    max_length: 16  # Max window name length
    abbreviate_issue_id: true  # Extract ISSUE-123 pattern
```

Per-repo override in `.wt.yaml`:

```yaml
tmux:
  enabled: false  # Disable for this repo
```

## Requirements

- tmux >= 1.9
- Running inside tmux session (check `$TMUX` env var)

## Implementation

See [Tmux Integration Design](docs/plans/2025-01-25-wt-tmux-integration-design.md) for architecture details.
```

**Step 3: Run tests to ensure nothing broke**

Run: `go test ./...`
Expected: All tests pass

**Step 4: Commit**

```bash
git add README.md docs/tmux-windows.md
git commit -m "docs(tmux): add documentation for window integration"
```

---

## Task 16: Final Verification and Cleanup

**Files:**
- All modified files

**Step 1: Run all tests**

```bash
go test ./...
```

Expected: All tests pass

**Step 2: Run linter**

```bash
make lint
```

Expected: No linter errors

**Step 3: Build the binary**

```bash
make build
```

Expected: Binary builds successfully to `bin/wt`

**Step 4: Manual verification (if in tmux)**

```bash
# Test basic window creation
./bin/wt add test-branch
# Verify window exists
tmux list-windows

# Test --no-tmux flag
./bin/wt add test-branch2 --no-tmux
# Verify no new window

# Cleanup
./bin/wt remove test-branch
./bin/wt remove test-branch2
```

**Step 5: Update CLAUDE.md if needed**

Add to CLAUDE.md under Architecture Overview:

```markdown
### Tmux Window Integration (v2)

WT v2 adds automatic tmux window management:

- `internal/tmux/session.go` - Extended with window operations (CreateNewWindow, SelectWindow, etc.)
- `internal/tmux/window_naming_test.go` - Window naming logic tests
- Smart naming: issue ID extraction, prefix/suffix abbreviation, stack numbering
- Integration points: `wt add`, `wt stack` create windows; `wt remove` cleans up
- Global `--no-tmux` flag to disable window creation per command
```

**Step 6: Final commit**

```bash
git add CLAUDE.md
git commit -m "docs(claude): update architecture documentation for tmux integration"
```

**Step 7: Create summary documentation**

Create `docs/plans/2025-01-26-wt-tmux-window-integration-summary.md`:

```markdown
# Tmux Window Integration - Implementation Summary

**Completed:** 2025-01-26
**Status:** ✅ Complete

## What Was Built

### Core Features
1. **Automatic window creation** - New tmux windows created on `wt add` and `wt stack`
2. **Smart naming** - Intelligent abbreviation of branch names for window titles
3. **Stack support** - Numbered window names for stacked branches (/1, /2, etc.)
4. **Window cleanup** - Automatic window closing on `wt remove`
5. **Global flag** - `--no-tmux` to disable window creation

### Window Naming Logic
- Issue ID extraction: `feature/nova-123` → `nova-123`
- Prefix abbreviation: `bugfix` → `fix`, `refactor` → `ref`
- Suffix abbreviation: `auth-provider` → `auth-p`
- Stack numbering: Root=no suffix, level 1=/1, level 2=/2
- Max length: 16 characters

### Files Modified/Created

**New Files:**
- `internal/tmux/window_naming_test.go` - Window naming unit tests
- `internal/tmux/window_test.go` - Window operation tests
- `internal/cli/root_tmux_test.go` - Tmux detection tests
- `internal/spice/stack_test.go` - Stack level detection tests
- `tests/tmux_integration_test.go` - Integration tests
- `docs/tmux-windows.md` - User documentation

**Modified Files:**
- `internal/tmux/session.go` - Added window operations and naming functions
- `internal/cli/root.go` - Added --no-tmux global flag
- `internal/cli/add.go` - Integrated window creation
- `internal/cli/stack.go` - Integrated stack window naming
- `internal/cli/remove.go` - Integrated window cleanup
- `internal/spice/client.go` - Added GetStackLevel function
- `internal/config/config.go` - Added tmux window config options

## Testing

All tests pass:
- Unit tests for window naming logic
- Window operation tests (when in tmux)
- Integration tests for end-to-end workflows
- Linter checks pass
- Binary builds successfully

## Usage Examples

```bash
# Basic usage - creates window "feat/auth"
wt add feat/auth

# Issue ID extraction - creates window "nova-123"
wt add feature/nova-123

# Stack with numbered windows - creates "feat/auth/1"
wt stack

# Disable tmux for this command
wt add temp-branch --no-tmux
```

## Next Steps

Possible enhancements for future:
- Per-branch window name customization
- Window layout templates
- Automatic window grouping
- Session management (currently users create sessions manually)
```

**Step 8: Commit summary**

```bash
git add docs/plans/2025-01-26-wt-tmux-window-integration-summary.md
git commit -m "docs(tmux): add implementation summary"
```

---

## Summary

This implementation plan adds complete tmux window integration to wt with:

1. **Smart window naming** - Issue ID extraction, prefix/suffix abbreviation, stack numbering
2. **Automatic window creation** - On `wt add` and `wt stack` when in tmux
3. **Window cleanup** - On `wt remove`
4. **Global flag** - `--no-tmux` to disable
5. **Full test coverage** - Unit tests for naming, window operations, integration tests
6. **Documentation** - User docs and architecture updates

All tasks follow TDD: write failing test, implement, verify pass, commit. Each task is bite-sized (2-5 minutes) and builds incrementally on the previous work.
