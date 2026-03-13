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

// ErrCanceled reports an explicit user cancellation.
var ErrCanceled = errors.New("selection canceled")

var (
	selectedStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
	matchStyle    = lipgloss.NewStyle().Bold(true)
)

// FuzzyItem is a selectable item in the fuzzy picker.
type FuzzyItem struct {
	Label string
	Value string
}

// FuzzySelect is a Bubble Tea model for fuzzy selection with pinned items.
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
	canceled  bool
}

type visibleItem struct {
	item           FuzzyItem
	matchedIndexes []int
}

// NewFuzzySelect constructs a fuzzy picker with the provided title and items.
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
		viewport:  viewport.New(0, smallerInt(height, largerInt(1, len(items)+len(pinned)))),
		height:    height,
	}
	model.refreshMatches()
	model.syncViewport()

	return model
}

// Init implements tea.Model.
func (m *FuzzySelect) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *FuzzySelect) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height - 3
		if m.height < 1 {
			m.height = 1
		}
		m.viewport.Width = msg.Width
		m.viewport.Height = smallerInt(m.height, largerInt(1, m.totalOptions()))
		m.syncViewport()
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEscape:
			m.canceled = true
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
		if m.totalOptions() == 0 {
			m.cursor = 0
		} else if m.cursor >= m.totalOptions() {
			m.cursor = m.totalOptions() - 1
		} else {
			m.cursor = 0
		}
		m.syncViewport()
		return m, cmd
	}

	return m, nil
}

// View implements tea.Model.
func (m *FuzzySelect) View() string {
	return strings.Join([]string{
		fmt.Sprintf("%s  %d/%d", m.title, len(m.visibleItems()), len(m.items)),
		m.textInput.View(),
		m.viewport.View(),
	}, "\n")
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
	entries := m.visibleEntries()
	items := make([]FuzzyItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry.item)
	}

	return items
}

func (m *FuzzySelect) visibleEntries() []visibleItem {
	if m.textInput.Value() == "" {
		items := make([]visibleItem, 0, len(m.items))
		for _, item := range m.items {
			items = append(items, visibleItem{item: item})
		}
		return items
	}

	items := make([]visibleItem, 0, len(m.matches))
	for _, match := range m.matches {
		items = append(items, visibleItem{
			item:           m.items[match.Index],
			matchedIndexes: append([]int(nil), match.MatchedIndexes...),
		})
	}

	return items
}

func (m *FuzzySelect) selectedItem() *FuzzyItem {
	if m.cursor < len(m.pinned) {
		return &m.pinned[m.cursor]
	}

	visible := m.visibleEntries()
	index := m.cursor - len(m.pinned)
	if index < 0 || index >= len(visible) {
		return nil
	}

	item := visible[index].item
	return &item
}

func (m *FuzzySelect) totalOptions() int {
	return len(m.pinned) + len(m.visibleItems())
}

// Run executes the picker program and returns the selected item.
func (m *FuzzySelect) Run(ctx context.Context) (*FuzzyItem, error) {
	if m.canceled {
		return nil, ErrCanceled
	}
	if m.chosen != nil {
		return m.chosen, nil
	}

	program := tea.NewProgram(m, tea.WithContext(ctx))
	finalModel, err := program.Run()
	if err != nil {
		return nil, fmt.Errorf("run fuzzy select: %w", err)
	}

	result, ok := finalModel.(*FuzzySelect)
	if !ok {
		return nil, fmt.Errorf("run fuzzy select: unexpected model type %T", finalModel)
	}
	if result.canceled {
		return nil, ErrCanceled
	}
	if result.chosen == nil {
		return nil, errors.New("fuzzy select finished without a selection")
	}

	return result.chosen, nil
}

func (m *FuzzySelect) syncViewport() {
	visible := m.visibleEntries()
	lines := make([]string, 0, len(m.pinned)+len(visible))

	for i, item := range m.pinned {
		line := "  " + item.Label
		if m.cursor == i {
			line = selectedStyle.Render("> " + item.Label)
		}
		lines = append(lines, line)
	}

	for i, entry := range visible {
		cursorIndex := len(m.pinned) + i
		renderedLabel := m.renderLabel(entry.item, entry.matchedIndexes)
		line := "  " + renderedLabel
		if m.cursor == cursorIndex {
			line = selectedStyle.Render("> " + renderedLabel)
		}
		lines = append(lines, line)
	}

	if m.viewport.Height == 0 {
		m.viewport.Height = smallerInt(m.height, largerInt(1, len(lines)))
	}

	m.viewport.SetContent(strings.Join(lines, "\n"))
	m.keepCursorVisible(len(lines))
}

func (m *FuzzySelect) renderLabel(item FuzzyItem, matchedIndexes []int) string {
	if len(matchedIndexes) == 0 {
		return item.Label
	}

	matched := make(map[int]struct{}, len(matchedIndexes))
	for _, idx := range matchedIndexes {
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

func (m *FuzzySelect) keepCursorVisible(totalLines int) {
	if totalLines == 0 || m.viewport.Height <= 0 {
		m.viewport.SetYOffset(0)
		return
	}
	if m.cursor < m.viewport.YOffset {
		m.viewport.SetYOffset(m.cursor)
		return
	}
	bottom := m.viewport.YOffset + m.viewport.Height - 1
	if m.cursor > bottom {
		m.viewport.SetYOffset(m.cursor - m.viewport.Height + 1)
	}
}

func smallerInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func largerInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
