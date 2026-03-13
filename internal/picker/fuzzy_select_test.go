package picker

import (
	"context"
	"errors"
	"testing"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	model.syncViewport()

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
