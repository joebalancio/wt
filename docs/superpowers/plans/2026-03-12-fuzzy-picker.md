# Fuzzy Picker Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `huh.Select` in the picker flow with a custom Bubble Tea fuzzy picker that keeps the filter input pinned and supports fuzzy matching for branches and worktrees.

**Architecture:** Add a self-contained `FuzzySelect` model in `internal/picker/fuzzy_select.go` that owns text input, fuzzy ranking, selection state, rendering, and Bubble Tea execution. Keep `Picker`’s public API unchanged, and add one small package-level test seam in `picker.go` so integration tests can verify item wiring without driving an interactive TUI.

**Tech Stack:** Go 1.22.2, Bubble Tea, Bubbles (`textinput`, `viewport`), Lip Gloss, `sahilm/fuzzy`, `huh` for single-line text entry only

---

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/picker/fuzzy_select.go` | New fuzzy picker model, filtering helpers, rendering, and `Run(context.Context)` |
| `internal/picker/fuzzy_select_test.go` | Unit tests for filtering, ranking, navigation, cancellation, rendering, and `Run` short-circuit behavior |
| `internal/picker/picker.go` | Replace `huh.Select` usage with `FuzzySelect`; add a narrow injectable runner seam for tests |
| `internal/picker/picker_test.go` | Verify `Picker` still handles git errors and now builds the right fuzzy picker inputs/results |
| `go.mod` | Add direct dependencies needed by the new picker |
| `go.sum` | Checksum updates from dependency changes |

## Design Constraints

- Keep `SelectWorktree(ctx)` and `SelectBranch(ctx)` signatures unchanged.
- Keep `promptNewBranch`’s name-entry step on `huh.NewInput()`.
- Return a package sentinel `ErrCancelled` from the custom picker on Esc / Ctrl+C.
- Empty filter must show all non-pinned items in original order.
- Pinned items must always render at the top and never participate in fuzzy matching.
- `Picker` tests must stay non-interactive.

## Chunk 1: Build and Verify the FuzzySelect Component

### Task 1: Add direct dependencies for the custom picker

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add the required direct dependencies**

Run:
```bash
go get github.com/sahilm/fuzzy@latest github.com/charmbracelet/bubbletea@v1.1.0 github.com/charmbracelet/bubbles@v0.20.0 github.com/charmbracelet/lipgloss@v0.13.0
```

Expected: `go.mod` gains direct `require` entries for the four packages.

- [ ] **Step 2: Tidy the module graph**

Run:
```bash
go mod tidy
```

Expected: command exits successfully with no error output.

- [ ] **Step 3: Verify the new dependency is present**

Run:
```bash
rg -n "sahilm/fuzzy|bubbletea|bubbles|lipgloss" go.mod
```

Expected: output shows all four packages in direct dependencies.

- [ ] **Step 4: Commit the dependency change**

```bash
git add go.mod go.sum
git commit -m "deps: add dependencies for fuzzy picker"
```

### Task 2: Create the FuzzySelect type and filtering helpers with tests first

**Files:**
- Create: `internal/picker/fuzzy_select.go`
- Create: `internal/picker/fuzzy_select_test.go`

- [ ] **Step 1: Write failing tests for constructor state and filter behavior**

Create `internal/picker/fuzzy_select_test.go` with:

```go
package picker

import (
	"testing"
)

func TestNewFuzzySelect_InitialState(t *testing.T) {
	items := []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
	}
	pinned := []FuzzyItem{
		{Label: "Create new branch", Value: newBranchOption},
	}

	model := NewFuzzySelect("Select branch:", items, pinned)

	if model.title != "Select branch:" {
		t.Fatalf("title = %q, want %q", model.title, "Select branch:")
	}
	if len(model.items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(model.items))
	}
	if len(model.pinned) != 1 {
		t.Fatalf("len(pinned) = %d, want 1", len(model.pinned))
	}
	if model.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", model.cursor)
	}
	if got := len(model.visibleItems()); got != 2 {
		t.Fatalf("visibleItems() len = %d, want 2", got)
	}
}

func TestFuzzySelect_VisibleItems_EmptyQueryPreservesOrder(t *testing.T) {
	model := NewFuzzySelect("Select:", []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
		{Label: "feat/auth", Value: "feat/auth"},
	}, nil)

	model.textInput.SetValue("")
	model.refreshMatches()

	got := model.visibleItems()
	if len(got) != 3 {
		t.Fatalf("len(visibleItems) = %d, want 3", len(got))
	}
	if got[0].Value != "main" || got[1].Value != "develop" || got[2].Value != "feat/auth" {
		t.Fatalf("visibleItems order = %#v", got)
	}
}

func TestFuzzySelect_VisibleItems_FuzzyMatchesQuery(t *testing.T) {
	model := NewFuzzySelect("Select:", []FuzzyItem{
		{Label: "feat/oauth-provider", Value: "feat/oauth-provider"},
		{Label: "feat/auth-api", Value: "feat/auth-api"},
		{Label: "bugfix/auth-token-refresh", Value: "bugfix/auth-token-refresh"},
	}, nil)

	model.textInput.SetValue("auth")
	model.refreshMatches()

	got := model.visibleItems()
	if len(got) == 0 {
		t.Fatal("visibleItems() returned no matches")
	}
	if got[0].Value != "feat/auth-api" {
		t.Fatalf("top match = %q, want %q", got[0].Value, "feat/auth-api")
	}
}

func TestFuzzySelect_VisibleItems_NoMatches(t *testing.T) {
	model := NewFuzzySelect("Select:", []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
	}, nil)

	model.textInput.SetValue("zzzzz")
	model.refreshMatches()

	if got := len(model.visibleItems()); got != 0 {
		t.Fatalf("len(visibleItems) = %d, want 0", got)
	}
}
```

Run:
```bash
go test ./internal/picker -run "TestNewFuzzySelect_InitialState|TestFuzzySelect_VisibleItems" -v
```

Expected: FAIL with `undefined: FuzzyItem`, `undefined: NewFuzzySelect`, and related missing methods.

- [ ] **Step 2: Implement the minimal type, constructor, and filter helpers**

Create `internal/picker/fuzzy_select.go` with:

```go
package picker

import (
	"context"
	"errors"
	"os"
	"sort"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
	"golang.org/x/term"
)

var ErrCancelled = errors.New("selection cancelled")

type FuzzyItem struct {
	Label string
	Value string
}

type FuzzySelect struct {
	title     string
	items     []FuzzyItem
	pinned    []FuzzyItem
	matches   []fuzzy.Match
	textInput textinput.Model
	viewport  viewport.Model
	cursor    int
	height    int
	chosen    *FuzzyItem
	cancelled bool
}

func NewFuzzySelect(title string, items []FuzzyItem, pinned []FuzzyItem) *FuzzySelect {
	input := textinput.New()
	input.Placeholder = "Type to filter..."
	input.Focus()

	height := 20
	if _, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && h > 3 {
		height = h - 3
	}

	model := &FuzzySelect{
		title:     title,
		items:     append([]FuzzyItem(nil), items...),
		pinned:    append([]FuzzyItem(nil), pinned...),
		textInput: input,
		height:    height,
	}
	model.refreshMatches()
	return model
}

func (m *FuzzySelect) Init() tea.Cmd {
	return nil
}

func (m *FuzzySelect) refreshMatches() {
	query := m.textInput.Value()
	if query == "" {
		m.matches = nil
		return
	}

	labels := make([]string, len(m.items))
	for i, item := range m.items {
		labels[i] = item.Label
	}

	m.matches = fuzzy.Find(query, labels)
	sort.SliceStable(m.matches, func(i, j int) bool {
		return m.matches[i].Score > m.matches[j].Score
	})
}

func (m *FuzzySelect) visibleItems() []FuzzyItem {
	if m.textInput.Value() == "" {
		return append([]FuzzyItem(nil), m.items...)
	}

	items := make([]FuzzyItem, 0, len(m.matches))
	for _, match := range m.matches {
		items = append(items, m.items[match.Index])
	}
	return items
}

func (m *FuzzySelect) selectedItem() *FuzzyItem {
	totalPinned := len(m.pinned)
	if m.cursor < totalPinned {
		return &m.pinned[m.cursor]
	}

	visible := m.visibleItems()
	index := m.cursor - totalPinned
	if index < 0 || index >= len(visible) {
		return nil
	}

	item := visible[index]
	return &item
}

func (m *FuzzySelect) totalOptions() int {
	return len(m.pinned) + len(m.visibleItems())
}

func (m *FuzzySelect) Run(ctx context.Context) (*FuzzyItem, error) {
	if m.cancelled {
		return nil, ErrCancelled
	}
	if m.chosen != nil {
		return m.chosen, nil
	}

	program := tea.NewProgram(m, tea.WithContext(ctx))
	finalModel, err := program.Run()
	if err != nil {
		return nil, err
	}

	result := finalModel.(*FuzzySelect)
	if result.cancelled {
		return nil, ErrCancelled
	}
	if result.chosen == nil {
		return nil, nil
	}
	return result.chosen, nil
}
```

Run:
```bash
go test ./internal/picker -run "TestNewFuzzySelect_InitialState|TestFuzzySelect_VisibleItems" -v
```

Expected: PASS

- [ ] **Step 3: Commit the component skeleton**

```bash
git add internal/picker/fuzzy_select.go internal/picker/fuzzy_select_test.go
git commit -m "feat(picker): add fuzzy select model skeleton"
```

### Task 3: Implement navigation, cancellation, rendering, and viewport behavior

**Files:**
- Modify: `internal/picker/fuzzy_select.go`
- Modify: `internal/picker/fuzzy_select_test.go`

- [ ] **Step 1: Add failing tests for update, view, and cancellation behavior**

Append to `internal/picker/fuzzy_select_test.go`:

```go
package picker

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFuzzySelect_Update_MovesCursorWithinBounds(t *testing.T) {
	model := NewFuzzySelect("Select:", []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
	}, nil)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := next.(*FuzzySelect)
	if updated.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", updated.cursor)
	}

	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated = next.(*FuzzySelect)
	if updated.cursor != 1 {
		t.Fatalf("cursor after second down = %d, want 1", updated.cursor)
	}

	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyUp})
	updated = next.(*FuzzySelect)
	if updated.cursor != 0 {
		t.Fatalf("cursor after up = %d, want 0", updated.cursor)
	}
}

func TestFuzzySelect_Update_EnterChoosesPinnedOrVisibleItem(t *testing.T) {
	model := NewFuzzySelect("Select:", []FuzzyItem{
		{Label: "main", Value: "main"},
	}, []FuzzyItem{
		{Label: "Create new branch", Value: newBranchOption},
	})

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(*FuzzySelect)
	if updated.chosen == nil {
		t.Fatal("chosen = nil, want selected pinned item")
	}
	if updated.chosen.Value != newBranchOption {
		t.Fatalf("chosen.Value = %q, want %q", updated.chosen.Value, newBranchOption)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want quit command")
	}
}

func TestFuzzySelect_Update_EscapeCancels(t *testing.T) {
	model := NewFuzzySelect("Select:", []FuzzyItem{
		{Label: "main", Value: "main"},
	}, nil)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	updated := next.(*FuzzySelect)
	if !updated.cancelled {
		t.Fatal("cancelled = false, want true")
	}
}

func TestFuzzySelect_Update_TextEntryRefreshesMatchesAndResetsCursor(t *testing.T) {
	model := NewFuzzySelect("Select:", []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "feat/auth", Value: "feat/auth"},
	}, nil)
	model.cursor = 1

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	updated := next.(*FuzzySelect)

	if updated.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", updated.cursor)
	}
	if got := len(updated.visibleItems()); got == 0 {
		t.Fatal("visibleItems() returned 0 matches after typing")
	}
}

func TestFuzzySelect_View_ShowsTitleCountInputAndPinnedItem(t *testing.T) {
	model := NewFuzzySelect("Select branch:", []FuzzyItem{
		{Label: "main", Value: "main"},
		{Label: "feat/auth", Value: "feat/auth"},
	}, []FuzzyItem{
		{Label: "Create new branch", Value: newBranchOption},
	})
	model.textInput.SetValue("auth")
	model.refreshMatches()

	view := model.View()
	for _, want := range []string{"Select branch:", "1/2", "auth", "Create new branch", "feat/auth"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q in %q", want, view)
		}
	}
}

func TestFuzzySelect_Run_ShortCircuitsChosenAndCancelled(t *testing.T) {
	model := NewFuzzySelect("Select:", []FuzzyItem{
		{Label: "main", Value: "main"},
	}, nil)
	model.chosen = &FuzzyItem{Label: "main", Value: "main"}

	item, err := model.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if item == nil || item.Value != "main" {
		t.Fatalf("Run() item = %#v, want main", item)
	}

	cancelled := NewFuzzySelect("Select:", []FuzzyItem{
		{Label: "main", Value: "main"},
	}, nil)
	cancelled.cancelled = true

	item, err = cancelled.Run(context.Background())
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("Run() err = %v, want ErrCancelled", err)
	}
	if item != nil {
		t.Fatalf("Run() item = %#v, want nil", item)
	}
}
```

Then fix the imports at the top of `internal/picker/fuzzy_select_test.go` to:

```go
import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)
```

Run:
```bash
go test ./internal/picker -run "TestFuzzySelect_Update|TestFuzzySelect_View|TestFuzzySelect_Run" -v
```

Expected: FAIL because `Update` and `View` are not implemented yet.

- [ ] **Step 2: Implement `Update`, viewport syncing, and `View`**

Update `internal/picker/fuzzy_select.go` to:

```go
package picker

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

var (
	selectedStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
	matchStyle    = lipgloss.NewStyle().Bold(true)
)

func (m *FuzzySelect) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height - 3
		if m.height < 1 {
			m.height = 1
		}
		m.viewport.Width = msg.Width
		m.viewport.Height = min(m.height, max(1, m.totalOptions()))
		m.syncViewport()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEscape:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyUp, tea.KeyCtrlP, tea.KeyCtrlK:
			if m.cursor > 0 {
				m.cursor--
			}
			m.syncViewport()
			return m, nil
		case tea.KeyDown, tea.KeyCtrlN, tea.KeyCtrlJ:
			if m.cursor < m.totalOptions()-1 {
				m.cursor++
			}
			m.syncViewport()
			return m, nil
		case tea.KeyEnter:
			m.chosen = m.selectedItem()
			return m, tea.Quit
		}

		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		m.refreshMatches()
		if m.cursor >= m.totalOptions() {
			m.cursor = max(0, m.totalOptions()-1)
		} else {
			m.cursor = 0
		}
		m.syncViewport()
		return m, cmd
	}

	return m, nil
}

func (m *FuzzySelect) View() string {
	var b strings.Builder

	title := fmt.Sprintf("%s  %d/%d", m.title, len(m.visibleItems()), len(m.items))
	b.WriteString(title)
	b.WriteByte('\n')
	b.WriteString(m.textInput.View())
	b.WriteByte('\n')
	b.WriteString(m.viewport.View())

	return b.String()
}

func (m *FuzzySelect) syncViewport() {
	visible := m.visibleItems()
	lines := make([]string, 0, len(m.pinned)+len(visible))

	for i, item := range m.pinned {
		line := "  " + item.Label
		if m.cursor == i {
			line = selectedStyle.Render("> " + item.Label)
		}
		lines = append(lines, line)
	}

	for i, item := range visible {
		cursorIndex := len(m.pinned) + i
		label := m.renderLabel(item)
		line := "  " + label
		if m.cursor == cursorIndex {
			line = selectedStyle.Render("> " + label)
		}
		lines = append(lines, line)
	}

	if m.viewport.Height == 0 {
		m.viewport.Height = min(m.height, max(1, len(lines)))
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))
}

func (m *FuzzySelect) renderLabel(item FuzzyItem) string {
	query := m.textInput.Value()
	if query == "" {
		return item.Label
	}

	matches := fuzzy.Find(query, []string{item.Label})
	if len(matches) == 0 {
		return item.Label
	}

	matched := map[int]struct{}{}
	for _, idx := range matches[0].MatchedIndexes {
		matched[idx] = struct{}{}
	}

	var b strings.Builder
	for i, r := range item.Label {
		if _, ok := matched[i]; ok {
			b.WriteString(matchStyle.Render(string(r)))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

Important cleanup while implementing:

- In `NewFuzzySelect`, initialize `model.viewport = viewport.New(0, min(height, max(1, len(items)+len(pinned))))`.
- Call `model.syncViewport()` before returning from `NewFuzzySelect`.
- In `Run`, wrap Bubble Tea failures as `fmt.Errorf("run fuzzy select: %w", err)`.
- Keep `ErrCancelled` untouched as the cancellation sentinel.

Run:
```bash
go test ./internal/picker -run "TestFuzzySelect_" -v
```

Expected: PASS

- [ ] **Step 3: Commit the finished component**

```bash
git add internal/picker/fuzzy_select.go internal/picker/fuzzy_select_test.go
git commit -m "feat(picker): implement fuzzy select behavior"
```

## Chunk 2: Integrate FuzzySelect into Picker and Verify End-to-End Wiring

### Task 4: Add a test seam for picker integration and cover non-interactive selection flows

**Files:**
- Modify: `internal/picker/picker.go`
- Modify: `internal/picker/picker_test.go`

- [ ] **Step 1: Write failing picker integration tests**

Update `internal/picker/picker_test.go` to:

```go
package picker

import (
	"context"
	"errors"
	"testing"

	"github.com/joebalancio/wt/pkg/domain"
)

func withStubbedFuzzyRunner(t *testing.T, stub func(context.Context, string, []FuzzyItem, []FuzzyItem) (*FuzzyItem, error)) {
	t.Helper()
	previous := runFuzzySelect
	runFuzzySelect = stub
	t.Cleanup(func() {
		runFuzzySelect = previous
	})
}

func TestPicker_SelectWorktree_UsesFuzzySelect(t *testing.T) {
	mock := &mockBranchLister{
		listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
			return []*domain.Worktree{
				{Path: "/repo", Branch: ""},
				{Path: "/repo/.worktrees/auth", Branch: "feat/auth"},
			}, nil
		},
	}
	picker := NewPicker(mock)

	withStubbedFuzzyRunner(t, func(_ context.Context, title string, items []FuzzyItem, pinned []FuzzyItem) (*FuzzyItem, error) {
		if title != "Select worktree to remove:" {
			t.Fatalf("title = %q, want %q", title, "Select worktree to remove:")
		}
		if len(pinned) != 0 {
			t.Fatalf("len(pinned) = %d, want 0", len(pinned))
		}
		if len(items) != 1 {
			t.Fatalf("len(items) = %d, want 1", len(items))
		}
		if items[0].Label != "feat/auth -> /repo/.worktrees/auth" {
			t.Fatalf("item label = %q", items[0].Label)
		}
		return &FuzzyItem{Label: items[0].Label, Value: items[0].Value}, nil
	})

	got, err := picker.SelectWorktree(context.Background())
	if err != nil {
		t.Fatalf("SelectWorktree() error = %v", err)
	}
	if got != "/repo/.worktrees/auth" {
		t.Fatalf("SelectWorktree() = %q, want %q", got, "/repo/.worktrees/auth")
	}
}

func TestPicker_SelectBranch_ReturnsExistingBranchFromFuzzySelect(t *testing.T) {
	mock := &mockBranchLister{
		listAllBranchesFunc: func(_ context.Context) ([]string, error) {
			return []string{"main", "develop"}, nil
		},
	}
	picker := NewPicker(mock)

	withStubbedFuzzyRunner(t, func(_ context.Context, title string, items []FuzzyItem, pinned []FuzzyItem) (*FuzzyItem, error) {
		if title != "Select or create a branch:" {
			t.Fatalf("title = %q, want %q", title, "Select or create a branch:")
		}
		if len(pinned) != 1 || pinned[0].Value != newBranchOption {
			t.Fatalf("pinned = %#v, want create-new option", pinned)
		}
		if len(items) != 2 {
			t.Fatalf("len(items) = %d, want 2", len(items))
		}
		return &FuzzyItem{Label: "develop", Value: "develop"}, nil
	})

	got, err := picker.SelectBranch(context.Background())
	if err != nil {
		t.Fatalf("SelectBranch() error = %v", err)
	}
	if got.Branch != "develop" || got.IsNew {
		t.Fatalf("SelectBranch() = %#v, want existing branch result", got)
	}
}

func TestPicker_SelectBranch_PropagatesCancellation(t *testing.T) {
	mock := &mockBranchLister{
		listAllBranchesFunc: func(_ context.Context) ([]string, error) {
			return []string{"main"}, nil
		},
	}
	picker := NewPicker(mock)

	withStubbedFuzzyRunner(t, func(_ context.Context, _ string, _ []FuzzyItem, _ []FuzzyItem) (*FuzzyItem, error) {
		return nil, ErrCancelled
	})

	_, err := picker.SelectBranch(context.Background())
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
}
```

Keep the existing error-path tests in the same file.

Run:
```bash
go test ./internal/picker -run "TestPicker_Select" -v
```

Expected: FAIL with `undefined: runFuzzySelect` or because `picker.go` still uses `huh.Select`.

- [ ] **Step 2: Add the minimal runner seam and switch `SelectWorktree` / `SelectBranch`**

Update `internal/picker/picker.go` to:

```go
package picker

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/joebalancio/wt/internal/git"
	"golang.org/x/term"
)

var runFuzzySelect = func(ctx context.Context, title string, items []FuzzyItem, pinned []FuzzyItem) (*FuzzyItem, error) {
	return NewFuzzySelect(title, items, pinned).Run(ctx)
}

func (p *Picker) SelectWorktree(ctx context.Context) (string, error) {
	worktrees, err := p.gitClient.ListWorktrees(ctx)
	if err != nil {
		return "", fmt.Errorf("list worktrees: %w", err)
	}

	if len(worktrees) == 0 {
		return "", fmt.Errorf("no worktrees found")
	}

	items := make([]FuzzyItem, 0, len(worktrees))
	for _, wt := range worktrees {
		if wt.Branch == "" {
			continue
		}
		items = append(items, FuzzyItem{
			Label: fmt.Sprintf("%s -> %s", wt.Branch, wt.Path),
			Value: wt.Path,
		})
	}

	if len(items) == 0 {
		return "", fmt.Errorf("no removable worktrees found")
	}

	selected, err := runFuzzySelect(ctx, "Select worktree to remove:", items, nil)
	if err != nil {
		return "", err
	}
	if selected == nil {
		return "", nil
	}
	return selected.Value, nil
}

func (p *Picker) SelectBranch(ctx context.Context) (SelectBranchResult, error) {
	branches, err := p.gitClient.ListAllBranches(ctx)
	if err != nil {
		return SelectBranchResult{}, fmt.Errorf("list branches: %w", err)
	}

	items := make([]FuzzyItem, len(branches))
	for i, branch := range branches {
		items[i] = FuzzyItem{Label: branch, Value: branch}
	}

	selected, err := runFuzzySelect(ctx, "Select or create a branch:", items, []FuzzyItem{
		{Label: newBranchOption, Value: newBranchOption},
	})
	if err != nil {
		return SelectBranchResult{}, err
	}
	if selected == nil {
		return SelectBranchResult{}, nil
	}
	if selected.Value == newBranchOption {
		return p.promptNewBranch(ctx, branches)
	}

	return SelectBranchResult{
		Branch: selected.Value,
		IsNew:  false,
	}, nil
}
```

Run:
```bash
go test ./internal/picker -run "TestPicker_Select" -v
```

Expected: PASS for `SelectWorktree` and existing-branch `SelectBranch` cases. `promptNewBranch` call will still not compile because its signature has not been updated yet.

- [ ] **Step 3: Commit the first picker integration change**

```bash
git add internal/picker/picker.go internal/picker/picker_test.go
git commit -m "feat(picker): use fuzzy select for branch and worktree pickers"
```

### Task 5: Switch the base-branch picker inside `promptNewBranch`

**Files:**
- Modify: `internal/picker/picker.go`
- Modify: `internal/picker/picker_test.go`

- [ ] **Step 1: Add a failing test for new-branch flow base selection**

Append to `internal/picker/picker_test.go`:

```go
func TestPicker_PromptNewBranch_UsesFuzzySelectForBaseBranch(t *testing.T) {
	mock := &mockBranchLister{}
	picker := NewPicker(mock)

	previousInput := runBranchNameInput
	runBranchNameInput = func(_ []string) (string, error) {
		return "feat/new-search", nil
	}
	t.Cleanup(func() {
		runBranchNameInput = previousInput
	})

	withStubbedFuzzyRunner(t, func(_ context.Context, title string, items []FuzzyItem, pinned []FuzzyItem) (*FuzzyItem, error) {
		if title != "Select base branch:" {
			t.Fatalf("title = %q, want %q", title, "Select base branch:")
		}
		if len(pinned) != 0 {
			t.Fatalf("len(pinned) = %d, want 0", len(pinned))
		}
		if len(items) != 2 {
			t.Fatalf("len(items) = %d, want 2", len(items))
		}
		return &FuzzyItem{Label: "develop", Value: "develop"}, nil
	})

	got, err := picker.promptNewBranch(context.Background(), []string{"main", "develop"})
	if err != nil {
		t.Fatalf("promptNewBranch() error = %v", err)
	}
	if got.Branch != "feat/new-search" || got.BaseBranch != "develop" || !got.IsNew {
		t.Fatalf("promptNewBranch() = %#v, want new branch result", got)
	}
}
```

Run:
```bash
go test ./internal/picker -run "TestPicker_PromptNewBranch_UsesFuzzySelectForBaseBranch" -v
```

Expected: FAIL because `promptNewBranch` does not accept `context.Context` and no branch-name input seam exists yet.

- [ ] **Step 2: Add the branch-name input seam and refactor `promptNewBranch`**

Update `internal/picker/picker.go` to:

```go
var runBranchNameInput = func(existingBranches []string) (string, error) {
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
		return "", err
	}
	return branchName, nil
}

func (p *Picker) promptNewBranch(ctx context.Context, existingBranches []string) (SelectBranchResult, error) {
	branchName, err := runBranchNameInput(existingBranches)
	if err != nil {
		return SelectBranchResult{}, err
	}

	items := make([]FuzzyItem, len(existingBranches))
	for i, branch := range existingBranches {
		items[i] = FuzzyItem{Label: branch, Value: branch}
	}

	selected, err := runFuzzySelect(ctx, "Select base branch:", items, nil)
	if err != nil {
		return SelectBranchResult{}, err
	}
	if selected == nil {
		return SelectBranchResult{}, nil
	}

	return SelectBranchResult{
		Branch:     branchName,
		BaseBranch: selected.Value,
		IsNew:      true,
	}, nil
}
```

Make sure the existing `SelectBranch` call site now passes `ctx` into `promptNewBranch`.

Run:
```bash
go test ./internal/picker -run "TestPicker_" -v
```

Expected: PASS

- [ ] **Step 3: Commit the base-branch integration**

```bash
git add internal/picker/picker.go internal/picker/picker_test.go
git commit -m "feat(picker): use fuzzy select for base branch choice"
```

### Task 6: Final verification, manual checks, and branch completion

**Files:**
- Verify all modified files

- [ ] **Step 1: Run focused picker tests**

Run:
```bash
go test ./internal/picker -v
```

Expected: all picker tests pass.

- [ ] **Step 2: Run the full project test suite**

Run:
```bash
make test
```

Expected: test suite passes.

- [ ] **Step 3: Run lint**

Run:
```bash
make lint
```

Expected: no lint failures.

- [ ] **Step 4: Build the binary**

Run:
```bash
make build
```

Expected: `bin/wt` is produced successfully.

- [ ] **Step 5: Do manual smoke checks**

Run:
```bash
./bin/wt --help
```

Expected: CLI help renders successfully.

Then manually exercise:

1. `wt add` in a repo with many branches.
2. Confirm the filter line stays visible while the result list scrolls.
3. Type `auth` and verify fuzzy hits include both `feat/auth-api` and `feat/oauth-provider`, with the more direct match first.
4. Press `Esc` or `Ctrl+C` and verify the command exits via `ErrCancelled` handling instead of a generic failure.

- [ ] **Step 6: Commit any remaining cleanup**

```bash
git status --short
git add go.mod go.sum internal/picker/fuzzy_select.go internal/picker/fuzzy_select_test.go internal/picker/picker.go internal/picker/picker_test.go
git commit -m "chore: finalize fuzzy picker implementation"
```

- [ ] **Step 7: Finish the development branch workflow**

Follow `superpowers:finishing-a-development-branch`, including:

1. verifying the branch is clean,
2. reviewing commit history,
3. completing any required issue-tracker updates,
4. `git pull --rebase`,
5. `bd sync`,
6. `git push`,
7. confirming `git status` reports the branch is up to date with origin.

## Testing Checklist

- [ ] `go test ./internal/picker -v`
- [ ] `make test`
- [ ] `make lint`
- [ ] `make build`
- [ ] Empty query shows all items in source order
- [ ] Pinned items stay visible above filtered results
- [ ] Fuzzy query ranks the closer auth branch first
- [ ] Esc / Ctrl+C returns `ErrCancelled`
- [ ] Large result sets keep the input visible while the viewport scrolls
- [ ] Base-branch selection in new-branch flow uses the custom picker

## Notes for the Implementer

- Do not remove `huh` from `go.mod`; it is still needed for `huh.NewInput()`.
- Keep the test seams (`runFuzzySelect`, `runBranchNameInput`) package-private and narrowly scoped. They exist only to keep tests deterministic.
- If `Run` returns `nil, nil` after `program.Run()`, treat that as an unexpected empty result and convert it to a concrete error during implementation instead of letting callers silently continue.
