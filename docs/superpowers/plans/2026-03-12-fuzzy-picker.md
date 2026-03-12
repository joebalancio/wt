# Fuzzy Picker Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `huh.Select` with a custom bubbletea `FuzzySelect` component that provides fuzzy matching and a pinned text input that never scrolls away.

**Architecture:** Create a self-contained `FuzzySelect` bubbletea model in `internal/picker/fuzzy_select.go` using `sahilm/fuzzy` for matching. The component manages text input, filtered list, and viewport internally. Integration points in `picker.go` remain unchanged — only the internal implementation switches from `huh.Select` to `FuzzySelect`.

**Tech Stack:** Go 1.22, bubbletea, bubbles (textinput, viewport), lipgloss, sahilm/fuzzy

---

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/picker/fuzzy_select.go` | **New** — FuzzySelect bubbletea component with pinned input, fuzzy matching, viewport |
| `internal/picker/fuzzy_select_test.go` | **New** — Unit tests for fuzzy matching logic, component state, cancellation |
| `internal/picker/picker.go` | **Modified** — Replace `huh.Select` calls with `FuzzySelect` in SelectBranch, SelectWorktree, promptNewBranch |
| `internal/picker/picker_test.go` | **Modified** — Add tests for FuzzySelect integration |
| `go.mod` | **Modified** — Add `sahilm/fuzzy` dependency |
| `go.sum` | **Modified** — Updated checksums |

---

## Chunk 1: Add Dependency and Create FuzzySelect Types

### Task 1: Add sahilm/fuzzy Dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add the fuzzy dependency**

Run:
```bash
cd /home/claude/projects/wt && go get github.com/sahilm/fuzzy@latest
```

Expected: `go: added github.com/sahilm/fuzzy vX.Y.Z`

- [ ] **Step 2: Verify the dependency was added**

Run:
```bash
grep "sahilm/fuzzy" go.mod
```

Expected: Line showing `github.com/sahilm/fuzzy` with version

- [ ] **Step 3: Tidy modules**

Run:
```bash
go mod tidy
```

Expected: No output (success)

- [ ] **Step 4: Commit the dependency change**

```bash
git add go.mod go.sum
git commit -m "deps: add sahilm/fuzzy for fuzzy matching in picker"
```

---

### Task 2: Create FuzzySelect Types and Constructor

**Files:**
- Create: `internal/picker/fuzzy_select.go`

- [ ] **Step 1: Write the failing test for FuzzyItem and constructor**

Create `internal/picker/fuzzy_select_test.go`:

```go
package picker

import (
	"testing"
)

func TestFuzzyItem(t *testing.T) {
	item := FuzzyItem{Label: "feat/auth", Value: "feat/auth"}
	if item.Label != "feat/auth" {
		t.Errorf("Label = %q, want %q", item.Label, "feat/auth")
	}
	if item.Value != "feat/auth" {
		t.Errorf("Value = %q, want %q", item.Value, "feat/auth")
	}
}

func TestNewFuzzySelect(t *testing.T) {
	items := []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
	}
	pinned := []FuzzyItem{
		{Label: "Create new branch", Value: "__new__"},
	}

	model := NewFuzzySelect("Select branch:", items, pinned)
	if model == nil {
		t.Fatal("NewFuzzySelect returned nil")
	}

	// Verify internal state via Init (should be non-nil tea.Model)
	cmd := model.Init()
	if cmd != nil {
		t.Error("Init() should return nil for this component")
	}
}
```

Run:
```bash
go test ./internal/picker -run "TestFuzzyItem|TestNewFuzzySelect" -v
```

Expected: FAIL — `undefined: FuzzyItem`, `undefined: NewFuzzySelect`

- [ ] **Step 2: Write minimal implementation**

Create `internal/picker/fuzzy_select.go`:

```go
// Package picker provides interactive TUI selection for wt commands.
package picker

import (
	"errors"
	"os"
	"sort"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"golang.org/x/term"
)

// ErrCancelled is returned when the user cancels selection (Esc/Ctrl+C).
// Callers can check for this to distinguish cancellation from other errors.
var ErrCancelled = errors.New("selection cancelled")

// FuzzyItem represents a single selectable item.
type FuzzyItem struct {
	Label string // display text (also used for fuzzy matching)
	Value string // return value (opaque to the component)
}

// FuzzySelect is a bubbletea model for fuzzy selection with a pinned text input.
type FuzzySelect struct {
	title     string
	items     []FuzzyItem
	pinned    []FuzzyItem
	filtered  []fuzzy.Match
	textInput textinput.Model
	cursor    int
	viewport  viewport.Model
	height    int
	chosen    *FuzzyItem
	cancelled bool
}

// NewFuzzySelect creates a new FuzzySelect model.
// items: the list of items to select from (participate in fuzzy matching)
// pinned: items always displayed at top (do not participate in matching)
func NewFuzzySelect(title string, items []FuzzyItem, pinned []FuzzyItem) *FuzzySelect {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Focus()

	// Get terminal height for viewport sizing
	height := 20 // default fallback
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && h > 3 {
		height = h - 3 // reserve lines for title, input, and padding
	}
	if w, _, _ := term.GetSize(int(os.Stdout.Fd())); w > 0 {
		ti.Width = w - 10 // leave room for prompt and padding
	}

	_ = w // silence unused variable warning

	model := &FuzzySelect{
		title:     title,
		items:     items,
		pinned:    pinned,
		textInput: ti,
		height:    height,
	}

	// Initial filter with empty input shows all items
	model.updateFilter()
	model.updateViewport()

	return model
}

// Init implements tea.Model.
func (m *FuzzySelect) Init() tea.Cmd {
	return nil
}

// updateFilter rebuilds the filtered list based on current input.
func (m *FuzzySelect) updateFilter() {
	input := m.textInput.Value()
	if input == "" {
		// Empty input: show all items in original order (no fuzzy matching)
		m.filtered = nil
		return
	}

	// Build slice of labels for fuzzy matching
	labels := make([]string, len(m.items))
	for i, item := range m.items {
		labels[i] = item.Label
	}

	// Perform fuzzy matching
	matches := fuzzy.Find(input, labels)

	// Sort by score descending (best matches first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	m.filtered = matches
}

// updateViewport recalculates viewport content and size.
func (m *FuzzySelect) updateViewport() {
	// Calculate number of visible items
	itemCount := len(m.pinned) + len(m.filtered)
	viewportHeight := min(itemCount, m.height)
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	m.viewport.Height = viewportHeight

	// Build viewport content
	var content string
	for i, item := range m.pinned {
		prefix := "  "
		if m.cursor == i {
			prefix = "> "
		}
		content += prefix + item.Label + "\n"
	}

	for i, match := range m.filtered {
		prefix := "  "
		if m.cursor == len(m.pinned)+i {
			prefix = "> "
		}
		content += prefix + m.renderMatch(match) + "\n"
	}

	m.viewport.SetContent(content)
}

// renderMatch renders a fuzzy match with highlighted characters.
func (m *FuzzySelect) renderMatch(match fuzzy.Match) string {
	label := m.items[match.MatchedIndex].Label

	// Build string with matched characters highlighted
	result := make([]rune, 0, len(label))
	matchedSet := make(map[int]bool)
	for _, idx := range match.MatchedIndexes {
		matchedSet[idx] = true
	}

	highlightStyle := lipgloss.NewStyle().Bold(true).Reverse(true)

	for i, r := range label {
		if matchedSet[i] {
			result = append(result, []rune(highlightStyle.Render(string(r)))...)
		} else {
			result = append(result, r)
		}
	}

	return string(result)
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Ensure FuzzySelect implements tea.Model at compile time.
var _ tea.Model = (*FuzzySelect)(nil)
```

Note: We declare `w` with underscore assignment to avoid unused variable errors while keeping the width calculation available for future use.

Run:
```bash
go test ./internal/picker -run "TestFuzzyItem|TestNewFuzzySelect" -v
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/picker/fuzzy_select.go internal/picker/fuzzy_select_test.go
git commit -m "feat(picker): add FuzzySelect types and constructor"
```

---

### Task 3: Implement Update Method

**Files:**
- Modify: `internal/picker/fuzzy_select.go`
- Modify: `internal/picker/fuzzy_select_test.go`

- [ ] **Step 1: Write the failing test for navigation and filtering**

Add to `internal/picker/fuzzy_select_test.go`:

```go
func TestFuzzySelect_Update_Navigation(t *testing.T) {
	items := []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
		{Label: "feat/auth", Value: "feat/auth"},
	}
	model := NewFuzzySelect("Select:", items, nil).(*FuzzySelect)

	// Initial cursor should be 0
	if model.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", model.cursor)
	}

	// Move down
	updatedModel, _ := model.Update(keyMsg("down"))
	model = updatedModel.(*FuzzySelect)
	if model.cursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", model.cursor)
	}

	// Move up (should stay at 0 due to bounds)
	updatedModel, _ = model.Update(keyMsg("up"))
	model = updatedModel.(*FuzzySelect)
	if model.cursor != 0 {
		t.Errorf("after up at top: cursor = %d, want 0", model.cursor)
	}
}

func TestFuzzySelect_Update_EnterSelects(t *testing.T) {
	items := []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
	}
	model := NewFuzzySelect("Select:", items, nil).(*FuzzySelect)

	// Press Enter to select first item
	updatedModel, cmd := model.Update(keyMsg("enter"))
	model = updatedModel.(*FuzzySelect)

	if model.chosen == nil {
		t.Error("Enter should set chosen item")
	}
	if model.chosen.Value != "main" {
		t.Errorf("chosen.Value = %q, want %q", model.chosen.Value, "main")
	}

	// Should return tea.Quit to exit the program
	if cmd == nil {
		t.Error("Enter should return tea.Quit command")
	}
}

func TestFuzzySelect_Update_EscCancels(t *testing.T) {
	items := []FuzzyItem{
		{Label: "main", Value: "main"},
	}
	model := NewFuzzySelect("Select:", items, nil).(*FuzzySelect)

	// Press Esc to cancel
	updatedModel, _ := model.Update(keyMsg("esc"))
	model = updatedModel.(*FuzzySelect)

	if !model.cancelled {
		t.Error("Esc should set cancelled flag")
	}
}

func TestFuzzySelect_Update_TextInputFilters(t *testing.T) {
	items := []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
		{Label: "feat/auth", Value: "feat/auth"},
	}
	model := NewFuzzySelect("Select:", items, nil).(*FuzzySelect)

	// Type "auth"
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model = updatedModel.(*FuzzySelect)
	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	model = updatedModel.(*FuzzySelect)
	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	model = updatedModel.(*FuzzySelect)
	updatedModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model = updatedModel.(*FuzzySelect)

	// Check that filtering occurred
	if len(model.filtered) == 0 {
		t.Error("typing 'auth' should filter to at least one item")
	}

	// Verify "feat/auth" is in results
	found := false
	for _, match := range model.filtered {
		if model.items[match.MatchedIndex].Label == "feat/auth" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'feat/auth' should be in filtered results for 'auth'")
	}
}

// keyMsg is a helper to create tea.KeyMsg for testing.
func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}
```

Run:
```bash
go test ./internal/picker -run "TestFuzzySelect_Update" -v
```

Expected: FAIL — Update method doesn't exist or doesn't work

- [ ] **Step 2: Implement Update method**

Add to `internal/picker/fuzzy_select.go` after `Init()`:

```go
import (
	"strings"
)

// Update implements tea.Model.
func (m *FuzzySelect) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp, tea.KeyCtrlP, tea.KeyCtrlK:
			// Move cursor up
			totalItems := len(m.pinned) + len(m.filtered)
			if m.cursor > 0 {
				m.cursor--
			}
			m.updateViewport()
			return m, nil

		case tea.KeyDown, tea.KeyCtrlN, tea.KeyCtrlJ:
			// Move cursor down
			totalItems := len(m.pinned) + len(m.filtered)
			if m.cursor < totalItems-1 {
				m.cursor++
			}
			m.updateViewport()
			return m, nil

		case tea.KeyEnter:
			// Select current item
			totalItems := len(m.pinned) + len(m.filtered)
			if totalItems == 0 {
				return m, tea.Quit
			}
			if m.cursor < len(m.pinned) {
				m.chosen = &m.pinned[m.cursor]
			} else if m.cursor < totalItems {
				matchIdx := m.cursor - len(m.pinned)
				m.chosen = &m.items[m.filtered[matchIdx].MatchedIndex]
			}
			return m, tea.Quit

		case tea.KeyEscape, tea.KeyCtrlC:
			// Cancel selection
			m.cancelled = true
			return m, tea.Quit

		case tea.KeyRunes:
			// Pass to text input
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			m.updateFilter()
			m.cursor = 0 // Reset cursor when filter changes
			m.updateViewport()
			return m, cmd

		case tea.KeyBackspace, tea.KeyDelete:
			// Pass to text input
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			m.updateFilter()
			m.cursor = 0
			m.updateViewport()
			return m, cmd
		}

	case tea.WindowSizeMsg:
		// Handle terminal resize
		m.height = msg.Height - 3
		if m.height < 1 {
			m.height = 1
		}
		m.textInput.Width = msg.Width - 10
		if m.textInput.Width < 10 {
			m.textInput.Width = 10
		}
		m.updateViewport()
		return m, nil
	}

	return m, nil
}
```

Also add the import for strings at the top (for potential future use):
```go
import (
	"errors"
	"os"
	"sort"
	"strings" // add this
)
```

Run:
```bash
go test ./internal/picker -run "TestFuzzySelect_Update" -v
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/picker/fuzzy_select.go internal/picker/fuzzy_select_test.go
git commit -m "feat(picker): implement FuzzySelect Update method with navigation and filtering"
```

---

### Task 4: Implement View Method and Run Function

**Files:**
- Modify: `internal/picker/fuzzy_select.go`
- Modify: `internal/picker/fuzzy_select_test.go`

- [ ] **Step 1: Write the failing test for View**

Add to `internal/picker/fuzzy_select_test.go`:

```go
func TestFuzzySelect_View(t *testing.T) {
	items := []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
	}
	model := NewFuzzySelect("Select branch:", items, nil).(*FuzzySelect)

	view := model.View()

	// View should contain title
	if !strings.Contains(view, "Select branch:") {
		t.Error("View should contain title")
	}

	// View should contain match count (2/2 for empty filter)
	if !strings.Contains(view, "2/") {
		t.Error("View should show match count")
	}

	// View should contain the items
	if !strings.Contains(view, "main") {
		t.Error("View should contain 'main'")
	}
}

func TestFuzzySelect_View_WithPinned(t *testing.T) {
	items := []FuzzyItem{
		{Label: "main", Value: "main"},
	}
	pinned := []FuzzyItem{
		{Label: "Create new branch", Value: "__new__"},
	}
	model := NewFuzzySelect("Select:", items, pinned).(*FuzzySelect)

	view := model.View()

	// Pinned item should be visible
	if !strings.Contains(view, "Create new branch") {
		t.Error("View should contain pinned item")
	}

	// Regular item should also be visible
	if !strings.Contains(view, "main") {
		t.Error("View should contain regular item")
	}
}
```

Also add the strings import to the test file:
```go
import (
	"strings"
	"testing"
)
```

Run:
```bash
go test ./internal/picker -run "TestFuzzySelect_View" -v
```

Expected: FAIL — View method doesn't exist

- [ ] **Step 2: Implement View method**

Add to `internal/picker/fuzzy_select.go`:

```go
// View implements tea.Model.
func (m *FuzzySelect) View() string {
	var b strings.Builder

	// Calculate match count (excludes pinned items)
	matchCount := len(m.filtered)
	totalCount := len(m.items)

	// Title with match count
	titleStyle := lipgloss.NewStyle().Bold(true)
	b.WriteString(titleStyle.Render(m.title))
	b.WriteString(fmt.Sprintf("              %d/%d\n", matchCount, totalCount))

	// Text input (always visible)
	b.WriteString(m.textInput.View())
	b.WriteString("\n")

	// List with cursor
	for i, item := range m.pinned {
		if m.cursor == i {
			b.WriteString(lipgloss.NewStyle().Bold(true).Render("> " + item.Label))
		} else {
			b.WriteString("  " + item.Label)
		}
		b.WriteString("\n")
	}

	for i, match := range m.filtered {
		idx := len(m.pinned) + i
		if m.cursor == idx {
			b.WriteString(lipgloss.NewStyle().Bold(true).Render("> " + m.renderMatch(match)))
		} else {
			b.WriteString("  " + m.renderMatch(match))
		}
		b.WriteString("\n")
	}

	return b.String()
}
```

Also add `fmt` to the imports:
```go
import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)
```

Run:
```bash
go test ./internal/picker -run "TestFuzzySelect_View" -v
```

Expected: PASS

- [ ] **Step 3: Write test for Run function**

Add to `internal/picker/fuzzy_select_test.go`:

```go
func TestFuzzySelect_Run_Cancelled(t *testing.T) {
	items := []FuzzyItem{
		{Label: "main", Value: "main"},
	}
	model := NewFuzzySelect("Select:", items, nil)
	model.cancelled = true

	result, err := model.Run(context.Background())
	if err != ErrCancelled {
		t.Errorf("Run with cancelled=true should return ErrCancelled, got %v", err)
	}
	if result != nil {
		t.Error("Run with cancelled=true should return nil result")
	}
}

func TestFuzzySelect_Run_Chosen(t *testing.T) {
	items := []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
	}
	model := NewFuzzySelect("Select:", items, nil).(*FuzzySelect)
	model.chosen = &items[0]

	result, err := model.Run(context.Background())
	if err != nil {
		t.Errorf("Run with chosen set should not error, got %v", err)
	}
	if result == nil {
		t.Fatal("Run with chosen set should return result")
	}
	if result.Value != "main" {
		t.Errorf("result.Value = %q, want %q", result.Value, "main")
	}
}
```

Add `context` import to test file:
```go
import (
	"context"
	"strings"
	"testing"
)
```

Run:
```bash
go test ./internal/picker -run "TestFuzzySelect_Run" -v
```

Expected: FAIL — Run method doesn't exist

- [ ] **Step 4: Implement Run method**

Add to `internal/picker/fuzzy_select.go`:

```go
import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"golang.org/x/term"
)

// Run executes the FuzzySelect and returns the selected item.
// Returns ErrCancelled if the user presses Esc or Ctrl+C.
func (m *FuzzySelect) Run(ctx context.Context) (*FuzzyItem, error) {
	// If already cancelled or chosen (for testing), return immediately
	if m.cancelled {
		return nil, ErrCancelled
	}
	if m.chosen != nil {
		return m.chosen, nil
	}

	// Run the bubbletea program with context for cancellation
	program := tea.NewProgram(m, tea.WithContext(ctx))
	finalModel, err := program.Run()
	if err != nil {
		return nil, fmt.Errorf("run picker: %w", err)
	}

	model := finalModel.(*FuzzySelect)

	if model.cancelled {
		return nil, ErrCancelled
	}

	if model.chosen == nil {
		return nil, errors.New("no selection made")
	}

	return model.chosen, nil
}
```

Run:
```bash
go test ./internal/picker -run "TestFuzzySelect_Run" -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/picker/fuzzy_select.go internal/picker/fuzzy_select_test.go
git commit -m "feat(picker): implement FuzzySelect View and Run methods"
```

---

### Task 5: Fuzzy Ranking Tests

**Files:**
- Modify: `internal/picker/fuzzy_select_test.go`

- [ ] **Step 1: Write tests for fuzzy ranking behavior**

Add to `internal/picker/fuzzy_select_test.go`:

```go
func TestFuzzySelect_Ranking(t *testing.T) {
	items := []FuzzyItem{
		{Label: "feat/oauth-provider", Value: "feat/oauth-provider"},
		{Label: "feat/auth-api", Value: "feat/auth-api"},
		{Label: "bugfix/auth-token-refresh", Value: "bugfix/auth-token-refresh"},
	}
	model := NewFuzzySelect("Select:", items, nil).(*FuzzySelect)

	// Type "auth"
	model.textInput.SetValue("auth")
	model.updateFilter()

	// Verify we have matches
	if len(model.filtered) == 0 {
		t.Fatal("'auth' should match at least one item")
	}

	// Verify feat/auth-api is in results (contains "auth" directly)
	found := false
	for _, match := range model.filtered {
		if model.items[match.MatchedIndex].Label == "feat/auth-api" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'feat/auth-api' should be in filtered results for 'auth'")
	}
}

func TestFuzzySelect_EmptyInputShowsAll(t *testing.T) {
	items := []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
		{Label: "feat/auth", Value: "feat/auth"},
	}
	model := NewFuzzySelect("Select:", items, nil).(*FuzzySelect)

	// Empty input means filtered is nil (show all in View)
	model.textInput.SetValue("")
	model.updateFilter()

	if model.filtered != nil {
		t.Error("empty input should result in nil filtered (show all)")
	}
}

func TestFuzzySelect_NoMatches(t *testing.T) {
	items := []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
	}
	model := NewFuzzySelect("Select:", items, nil).(*FuzzySelect)

	// Type something that won't match
	model.textInput.SetValue("zzzzz")
	model.updateFilter()

	if len(model.filtered) != 0 {
		t.Errorf("no matches expected, got %d", len(model.filtered))
	}
}

func TestFuzzySelect_SpecialCharacters(t *testing.T) {
	items := []FuzzyItem{
		{Label: "fix/issue#123", Value: "fix/issue#123"},
		{Label: "feat/api-v2", Value: "feat/api-v2"},
	}
	model := NewFuzzySelect("Select:", items, nil).(*FuzzySelect)

	// Type "issue"
	model.textInput.SetValue("issue")
	model.updateFilter()

	if len(model.filtered) == 0 {
		t.Error("'issue' should match 'fix/issue#123'")
	}
}

func TestFuzzySelect_MatchCountAccuracy(t *testing.T) {
	items := []FuzzyItem{
		{Label: "feat/auth", Value: "feat/auth"},
		{Label: "feat/oauth", Value: "feat/oauth"},
		{Label: "main", Value: "main"},
	}
	pinned := []FuzzyItem{
		{Label: "Create new", Value: "__new__"},
	}
	model := NewFuzzySelect("Select:", items, pinned).(*FuzzySelect)

	// Type "auth"
	model.textInput.SetValue("auth")
	model.updateFilter()

	// Count should be 2 (auth matches), not 3 (excludes pinned)
	matchCount := len(model.filtered)
	if matchCount != 2 {
		t.Errorf("match count = %d, want 2", matchCount)
	}
}
```

Run:
```bash
go test ./internal/picker -run "TestFuzzySelect_Ranking|TestFuzzySelect_Empty|TestFuzzySelect_NoMatches|TestFuzzySelect_Special|TestFuzzySelect_MatchCount" -v
```

Expected: PASS

- [ ] **Step 2: Commit**

```bash
git add internal/picker/fuzzy_select_test.go
git commit -m "test(picker): add fuzzy ranking and edge case tests"
```

---

## Chunk 2: Integrate FuzzySelect into Picker

### Task 6: Refactor SelectWorktree to use FuzzySelect

**Files:**
- Modify: `internal/picker/picker.go`
- Modify: `internal/picker/picker_test.go`

- [ ] **Step 1: Update SelectWorktree implementation**

Modify `internal/picker/picker.go`:

Replace the `SelectWorktree` method:

```go
// SelectWorktree presents a picker for selecting a worktree to remove.
// Returns the selected worktree path, or an error if selection fails.
func (p *Picker) SelectWorktree(ctx context.Context) (string, error) {
	worktrees, err := p.gitClient.ListWorktrees(ctx)
	if err != nil {
		return "", fmt.Errorf("list worktrees: %w", err)
	}

	if len(worktrees) == 0 {
		return "", fmt.Errorf("no worktrees found")
	}

	var items []FuzzyItem
	for _, wt := range worktrees {
		if wt.Branch == "" {
			continue
		}
		label := fmt.Sprintf("%s -> %s", wt.Branch, wt.Path)
		items = append(items, FuzzyItem{Label: label, Value: wt.Path})
	}

	if len(items) == 0 {
		return "", fmt.Errorf("no removable worktrees found")
	}

	result, err := NewFuzzySelect("Select worktree to remove:", items, nil).Run(ctx)
	if err != nil {
		return "", err
	}

	return result.Value, nil
}
```

Run:
```bash
go build ./...
```

Expected: Build succeeds

- [ ] **Step 2: Update test for SelectWorktree**

Modify `internal/picker/picker_test.go`. The existing tests for list errors and empty worktrees should still work, but add a test for FuzzySelect integration:

```go
// Note: Interactive selection tests cannot be fully automated.
// The existing error handling tests (list errors, empty worktrees) remain valid.
// FuzzySelect component is tested separately in fuzzy_select_test.go.
```

Run:
```bash
go test ./internal/picker -run "TestPicker_SelectWorktree" -v
```

Expected: PASS (existing error handling tests)

- [ ] **Step 3: Commit**

```bash
git add internal/picker/picker.go
git commit -m "feat(picker): replace huh.Select with FuzzySelect in SelectWorktree"
```

---

### Task 7: Refactor SelectBranch to use FuzzySelect

**Files:**
- Modify: `internal/picker/picker.go`

- [ ] **Step 1: Update SelectBranch implementation**

Modify `internal/picker/picker.go`:

Replace the `SelectBranch` method:

```go
// SelectBranch presents a picker for selecting or creating a branch.
func (p *Picker) SelectBranch(ctx context.Context) (SelectBranchResult, error) {
	branches, err := p.gitClient.ListAllBranches(ctx)
	if err != nil {
		return SelectBranchResult{}, fmt.Errorf("list branches: %w", err)
	}

	// Pinned item for "Create new branch"
	pinned := []FuzzyItem{{Label: newBranchOption, Value: newBranchOption}}

	// Regular items for existing branches
	items := make([]FuzzyItem, len(branches))
	for i, branch := range branches {
		items[i] = FuzzyItem{Label: branch, Value: branch}
	}

	result, err := NewFuzzySelect("Select or create a branch:", items, pinned).Run(ctx)
	if err != nil {
		return SelectBranchResult{}, err
	}

	if result.Value == newBranchOption {
		return p.promptNewBranch(branches)
	}

	return SelectBranchResult{
		Branch: result.Value,
		IsNew:  false,
	}, nil
}
```

Run:
```bash
go build ./...
```

Expected: Build succeeds

- [ ] **Step 2: Verify existing tests still pass**

Run:
```bash
go test ./internal/picker -run "TestPicker_SelectBranch" -v
```

Expected: PASS (existing error handling test)

- [ ] **Step 3: Commit**

```bash
git add internal/picker/picker.go
git commit -m "feat(picker): replace huh.Select with FuzzySelect in SelectBranch"
```

---

### Task 8: Refactor promptNewBranch Base Picker to use FuzzySelect

**Files:**
- Modify: `internal/picker/picker.go`

- [ ] **Step 1: Update promptNewBranch implementation**

Modify `internal/picker/picker.go`:

Replace the `promptNewBranch` method:

```go
func (p *Picker) promptNewBranch(existingBranches []string) (SelectBranchResult, error) {
	var branchName string
	err := huh.NewInput().
		Title("Enter new branch name:").
		Value(&branchName).
		Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("branch name cannot be empty")
			}
			for _, branch := range existingBranches {
				if branch == s {
					return fmt.Errorf("branch %q already exists", s)
				}
			}
			return nil
		}).
		Run()
	if err != nil {
		return SelectBranchResult{}, err
	}

	// Convert branches to FuzzyItems
	items := make([]FuzzyItem, len(existingBranches))
	for i, branch := range existingBranches {
		items[i] = FuzzyItem{Label: branch, Value: branch}
	}

	result, err := NewFuzzySelect("Select base branch:", items, nil).Run(ctx)
	if err != nil {
		return SelectBranchResult{}, err
	}

	return SelectBranchResult{
		Branch:     branchName,
		BaseBranch: result.Value,
		IsNew:      true,
	}, nil
}
```

Note: We need to add `ctx` parameter to `promptNewBranch`. Update the signature and call site.

- [ ] **Step 2: Fix the ctx parameter issue**

Actually, looking at the code, `promptNewBranch` doesn't have access to `ctx`. We need to either:
1. Pass ctx as a parameter
2. Store ctx in the Picker struct

Let's pass it as a parameter for minimal changes:

Update the method signature:
```go
func (p *Picker) promptNewBranch(ctx context.Context, existingBranches []string) (SelectBranchResult, error) {
```

And update the call site in `SelectBranch`:
```go
return p.promptNewBranch(ctx, branches)
```

Update the full method:

```go
func (p *Picker) promptNewBranch(ctx context.Context, existingBranches []string) (SelectBranchResult, error) {
	var branchName string
	err := huh.NewInput().
		Title("Enter new branch name:").
		Value(&branchName).
		Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("branch name cannot be empty")
			}
			for _, branch := range existingBranches {
				if branch == s {
					return fmt.Errorf("branch %q already exists", s)
				}
			}
			return nil
		}).
		Run()
	if err != nil {
		return SelectBranchResult{}, err
	}

	// Convert branches to FuzzyItems
	items := make([]FuzzyItem, len(existingBranches))
	for i, branch := range existingBranches {
		items[i] = FuzzyItem{Label: branch, Value: branch}
	}

	result, err := NewFuzzySelect("Select base branch:", items, nil).Run(ctx)
	if err != nil {
		return SelectBranchResult{}, err
	}

	return SelectBranchResult{
		Branch:     branchName,
		BaseBranch: result.Value,
		IsNew:      true,
	}, nil
}
```

Run:
```bash
go build ./...
```

Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add internal/picker/picker.go
git commit -m "feat(picker): replace huh.Select with FuzzySelect in promptNewBranch base picker"
```

---

### Task 9: Remove Unused huh Import

**Files:**
- Modify: `internal/picker/picker.go`

- [ ] **Step 1: Update imports**

We still use `huh.NewInput` for the branch name input, so we keep the `huh` import. Verify imports are correct.

Check current imports in `picker.go`:
```go
import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/joebalancio/wt/internal/git"
	"golang.org/x/term"
)
```

The `huh` import is still needed for `huh.NewInput`. No changes needed.

Run:
```bash
go build ./...
```

Expected: Build succeeds

---

### Task 10: Final Verification and Cleanup

**Files:**
- All modified files

- [ ] **Step 1: Run all picker tests**

Run:
```bash
go test ./internal/picker -v
```

Expected: All tests PASS

- [ ] **Step 2: Run full test suite**

Run:
```bash
make test
```

Expected: All tests PASS

- [ ] **Step 3: Run linter**

Run:
```bash
make lint
```

Expected: No errors

- [ ] **Step 4: Build the binary**

Run:
```bash
make build
```

Expected: Binary created at `bin/wt`

- [ ] **Step 5: Manual smoke test**

Run:
```bash
./bin/wt --help
```

Expected: Help output displayed

- [ ] **Step 6: Commit any remaining changes**

```bash
git status
git add -A
git commit -m "chore: final cleanup for fuzzy picker implementation"
```

- [ ] **Step 7: Create summary commit**

```bash
git log --oneline -10
```

Verify the commits are logical and atomic.

---

## Testing Checklist

After implementation, verify:

- [ ] **Unit tests pass** — `go test ./internal/picker -v`
- [ ] **Integration tests pass** — `make test`
- [ ] **Linter passes** — `make lint`
- [ ] **Binary builds** — `make build`
- [ ] **Small list (<10 items)** — Filter input visible, no unnecessary scrolling
- [ ] **Large list (100+ items)** — Filter input stays pinned, viewport scrolls
- [ ] **Fuzzy matching** — `auth` matches `feat/auth-api` higher than `feat/oauth-provider`
- [ ] **Pinned items** — "Create new branch" always visible
- [ ] **Cancellation** — Esc/Ctrl+C returns `ErrCancelled`
- [ ] **Terminal resize** — Viewport adapts to window size

---

## Rollback Plan

If issues arise:

1. **Revert commits:**
   ```bash
   git revert HEAD~N  # where N is number of commits
   ```

2. **Or restore from backup branch:**
   ```bash
   git checkout backup-branch
   ```

3. **Specific file rollback:**
   ```bash
   git checkout HEAD~1 -- internal/picker/picker.go
   ```
