# Fuzzy Picker for Branch/Worktree Selection

**Date:** 2026-03-12
**Status:** Approved

## Problem

The current `wt add` and `wt remove` interactive pickers use `huh.Select`, which has two issues:

1. **No fuzzy matching** — the built-in `/` filter uses `strings.Contains` (exact substring). Typing `auth` won't match `feat/oauth-provider`.
2. **Filter input hidden on large lists** — when the option list exceeds the viewport, the text input scrolls out of view, making it impossible to see what you're typing.

These problems compound in collaborative repos with 100+ branches.

## Solution

Replace `huh.Select` with a custom bubbletea `FuzzySelect` component that provides fuzzy matching with a pinned text input.

## Design

### FuzzySelect Component

**New file:** `internal/picker/fuzzy_select.go`

A self-contained bubbletea model with:

- **Text input** pinned at the top (never scrolls away)
- **Filtered list** below, ranked by fuzzy match score via `sahilm/fuzzy`
- **Viewport** for the list portion only

**Key types:**

```go
// ErrCancelled is returned when the user cancels selection (Esc/Ctrl+C).
// Callers can check for this to distinguish cancellation from other errors.
var ErrCancelled = errors.New("selection cancelled")

type FuzzySelect struct {
    title      string
    items      []FuzzyItem
    pinned     []FuzzyItem       // always-visible items (e.g. "Create new branch")
    filtered   []fuzzy.Match
    textInput  textinput.Model
    cursor     int
    viewport   viewport.Model
    height     int
    chosen     bool
    cancelled  bool
}

type FuzzyItem struct {
    Label string  // display text (also used for fuzzy matching)
    Value string  // return value (opaque to the component)
}
```

**Context cancellation:**

The `FuzzySelect` is run via `tea.NewProgram(model, tea.WithContext(ctx)).Run()`. This threads the caller's `context.Context` through to bubbletea, so context cancellation (e.g. timeouts, signals) stops the TUI cleanly.

**Fuzzy matching target:**

Fuzzy matching operates on `FuzzyItem.Label` only. For worktrees, the label is `"branch -> path"`, so both branch name and path are searchable. The `Value` field is opaque — used only as the return value after selection.

**Pinned items:**

Items in the `pinned` slice are always displayed at the top of the list regardless of filter state. They do not participate in fuzzy matching. This is used for synthetic options like "Create new branch" that must remain accessible even when filtering.

**Behavior:**

- Always in filter mode — typing immediately filters (no `/` prefix needed)
- Empty input shows all options in original order
- Fuzzy matching ranks results by score (e.g. `auth` ranks `feat/auth-api` above `feat/oauth-provider`)
- Match count displayed in header (e.g. "3/127") — pinned items excluded from count
- Matched characters highlighted within labels

**Cancellation:**

When the user presses Esc or Ctrl+C, `FuzzySelect` returns `ErrCancelled`. Callers should check for this sentinel error to distinguish user cancellation from unexpected failures. This replaces the current `huh.ErrUserAborted` behavior.

**Navigation keybindings:**

| Key | Action |
|-----|--------|
| Up / Ctrl+p / Ctrl+k | Move selection up |
| Down / Ctrl+n / Ctrl+j | Move selection down |
| Enter | Confirm selection |
| Esc / Ctrl+C | Cancel |

No bare `j/k` — conflicts with text input which is always active.

**Visual layout:**

```
Select branch:              3/127
> auth
  feat/auth-api
  feat/oauth-provider
  bugfix/auth-token-refresh
```

- Title + match count on first line
- Text input on second line (always visible)
- Filtered results below in a scrollable viewport
- Selected item: bold/reverse video via lipgloss
- Matched characters: highlighted within labels
- Viewport height: `min(filtered_count, terminal_height - 3)`
- Terminal height obtained via `term.GetSize(int(os.Stdout.Fd()))` (already imported for `IsTerminal()`)
- Fallback: if terminal size cannot be determined, default to 20 lines

### Integration with Picker

**Modified file:** `internal/picker/picker.go`

All three selection points switch from `huh.Select` to `FuzzySelect`:

1. **`SelectBranch`** — branch list + "Create new branch" option → FuzzySelect
2. **`SelectWorktree`** — worktree list (`branch -> path`) → FuzzySelect
3. **`promptNewBranch` base branch picker** — branch list → FuzzySelect

**Unchanged:**
- `promptNewBranch` text input — continues using `huh.Input` (works fine for single text entry)
- `Picker` struct, constructor, and `IsTerminal()` — no changes
- Public API (`SelectBranchResult`, method signatures) — no changes
- CLI command files (`add.go`, `remove.go`, etc.) — no changes needed

### Dependencies

**Add:** `github.com/sahilm/fuzzy` (latest) — small fuzzy matching library, no transitive dependencies.

**Keep:** `github.com/charmbracelet/huh` — still used for `huh.Input` in `promptNewBranch`.

**Promote to direct:** `bubbletea`, `bubbles`, `lipgloss` — currently indirect deps via `huh`. Building a custom bubbletea model imports these directly, promoting them to direct dependencies in `go.mod`. This is a cosmetic change to the dependency graph with no functional impact.

## Testing

### Unit Tests (`internal/picker/fuzzy_select_test.go`)

- Fuzzy ranking: `auth` matches `feat/auth-api` higher than `feat/oauth-provider`
- Empty input returns all items in original order
- No matches returns empty list
- Special characters in branch names (e.g. `fix/issue#123`)
- Match count accuracy

### Integration Tests (`internal/picker/picker_test.go`)

- `SelectBranch` returns correct `SelectBranchResult` shape
- `SelectWorktree` returns path string
- "Create new branch" option present and selectable

### Manual Testing

- Small list (<10) — snappy, no unnecessary viewport
- Large list (100+) — filter input stays visible, scrolling works
- Terminal resize — viewport adapts
- Ctrl+C cancellation returns error

No integration tests against real git — picker accepts `BranchLister` interface, mocked in tests.

## Files Changed

| File | Change |
|------|--------|
| `internal/picker/fuzzy_select.go` | **New** — FuzzySelect bubbletea component |
| `internal/picker/fuzzy_select_test.go` | **New** — unit tests for fuzzy matching and component |
| `internal/picker/picker.go` | **Modified** — replace `huh.Select` with `FuzzySelect` in all methods |
| `internal/picker/picker_test.go` | **Modified** — update tests for new component |
| `go.mod` | **Modified** — add `sahilm/fuzzy` |
| `go.sum` | **Modified** — updated checksums |
