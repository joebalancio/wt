package tmux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Session represents a tmux session
type Session struct {
	ID   string
	Name string
}

// Client wraps tmux operations
type Client struct {
	tmuxPath string
}

// NewClient creates a new tmux client
func NewClient() (*Client, error) {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return nil, fmt.Errorf("tmux not found in PATH: %w", err)
	}
	return &Client{tmuxPath: path}, nil
}

// ListSessions returns all tmux sessions
func (c *Client) ListSessions() ([]Session, error) {
	var stdout bytes.Buffer
	cmd := exec.Command(c.tmuxPath, "list-sessions", "-F", "#{session_id} #{session_name}")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// tmux returns error if no sessions exist
		if strings.Contains(err.Error(), "no server running") {
			return []Session{}, nil
		}
		return nil, fmt.Errorf("listing sessions: %w", err)
	}

	return parseSessionList(stdout.String())
}

// HasSession checks if a session with the given name exists
func (c *Client) HasSession(name string) (bool, error) {
	sessions, err := c.ListSessions()
	if err != nil {
		return false, err
	}

	for _, s := range sessions {
		if s.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// CreateSession creates a new tmux session
func (c *Client) CreateSession(name, path, layout, windowName string, attach bool) error {
	args := []string{"new-session", "-d", "-s", name, "-c", path, "-n", windowName}

	cmd := exec.Command(c.tmuxPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating session: %w", err)
	}

	if layout != "" {
		cmd = exec.Command(c.tmuxPath, "select-layout", "-t", name, layout)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("setting layout: %w", err)
		}
	}

	if attach {
		return c.AttachSession(name)
	}

	return nil
}

// AttachSession attaches to an existing session
func (c *Client) AttachSession(name string) error {
	cmd := exec.Command(c.tmuxPath, "attach-session", "-t", name)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// This typically replaces the current process
	return cmd.Run()
}

// KillSession kills a tmux session
func (c *Client) KillSession(name string) error {
	cmd := exec.Command(c.tmuxPath, "kill-session", "-t", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("killing session: %w", err)
	}
	return nil
}

func parseSessionList(output string) ([]Session, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	sessions := make([]Session, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		sessions = append(sessions, Session{
			ID:   parts[0],
			Name: parts[1],
		})
	}

	return sessions, nil
}

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

// truncate truncates a string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

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

// abbreviateSuffix takes first character of each word (words split by non-alphanumeric)
// Keeps the first word intact, abbreviates subsequent words
// Keeps digits-only words intact
func abbreviateSuffix(suffix string) string {
	// Split on non-alphanumeric: auth-provider → auth provider
	words := regexp.MustCompile(`[^a-zA-Z0-9]+`).Split(suffix, -1)

	// Filter out empty strings (from leading/multiple separators)
	var filtered []string
	for _, word := range words {
		if word != "" {
			filtered = append(filtered, word)
		}
	}

	if len(filtered) == 0 {
		return ""
	}

	// First word stays intact
	result := filtered[0]

	// Subsequent words: first char only (unless all digits)
	for i := 1; i < len(filtered); i++ {
		word := filtered[i]
		// If word is all digits, keep it
		if regexp.MustCompile(`^\d+$`).MatchString(word) {
			result += "-" + word
		} else {
			// Take first char
			result += "-" + string(word[0])
		}
	}

	return result
}

// GenerateWindowName generates a tmux window name from a branch name
func GenerateWindowName(branch string) string {
	// 1. Try issue ID extraction: feature/nova-123 → nova-123
	if issue := extractIssueID(branch); issue != "" {
		return truncate(issue, 16)
	}

	// 2. Parse branch components
	parts := strings.Split(branch, "/")

	var prefix, suffix string
	var isMultiPartBranch bool // Track if original branch has 3+ parts
	if len(parts) >= 2 {
		prefix = abbreviatePrefix(parts[0]) // feature → feat
		suffix = parts[len(parts)-1]        // take LAST part: feat/team/auth-api → auth-api
		isMultiPartBranch = len(parts) >= 3
	} else {
		suffix = branch
	}

	// 3. Abbreviate suffix based on context
	// Split on non-alphanumeric
	words := regexp.MustCompile(`[^a-zA-Z0-9]+`).Split(suffix, -1)

	// Filter out empty strings
	var filtered []string
	for _, word := range words {
		if word != "" {
			filtered = append(filtered, word)
		}
	}

	var abbreviated string
	if len(filtered) == 0 {
		abbreviated = ""
	} else if len(filtered) == 1 {
		// Single word: keep it intact
		abbreviated = filtered[0]
	} else if prefix == "" && len(filtered) >= 3 {
		// No prefix (e.g., "very-long-branch-name-here"): keep first 2 words,
		// take first 2 chars of 3rd word, drop rest
		abbreviated = filtered[0] + "-" + filtered[1]
		// Take first 2 chars of third word, or full word if shorter
		thirdWord := filtered[2]
		if len(thirdWord) >= 2 {
			abbreviated += "-" + string(thirdWord[0]) + string(thirdWord[1])
		} else {
			abbreviated += "-" + thirdWord
		}
	} else if len(filtered) == 2 {
		// Two words
		first := filtered[0]
		second := filtered[1]

		// If multi-part branch (3+ parts), abbreviate both words aggressively
		if isMultiPartBranch {
			var firstAbbr, secondAbbr string
			if regexp.MustCompile(`^\d+$`).MatchString(first) {
				firstAbbr = first
			} else {
				firstAbbr = string(first[0])
			}
			if regexp.MustCompile(`^\d+$`).MatchString(second) {
				secondAbbr = second
			} else {
				secondAbbr = string(second[0])
			}
			abbreviated = firstAbbr + "-" + secondAbbr
		} else if len(first) >= 4 {
			// Keep first intact, abbreviate second
			if regexp.MustCompile(`^\d+$`).MatchString(second) {
				abbreviated = first + "-" + second
			} else {
				abbreviated = first + "-" + string(second[0])
			}
		} else {
			// First word < 4 chars: abbreviate both
			var firstAbbr, secondAbbr string
			if regexp.MustCompile(`^\d+$`).MatchString(first) {
				firstAbbr = first
			} else {
				firstAbbr = string(first[0])
			}
			if regexp.MustCompile(`^\d+$`).MatchString(second) {
				secondAbbr = second
			} else {
				secondAbbr = string(second[0])
			}
			abbreviated = firstAbbr + "-" + secondAbbr
		}
	} else {
		// Three+ words with prefix: keep first TWO intact, abbreviate rest
		abbreviated = filtered[0] + "-" + filtered[1]
		for i := 2; i < len(filtered); i++ {
			word := filtered[i]
			if regexp.MustCompile(`^\d+$`).MatchString(word) {
				abbreviated += "-" + word
			} else {
				abbreviated += "-" + string(word[0])
			}
		}
	}

	// 4. Combine and truncate
	var result string
	if prefix != "" {
		result = fmt.Sprintf("%s/%s", prefix, abbreviated)
	} else {
		result = abbreviated
	}

	return truncate(result, 16)
}

// getStackRoot returns the root branch name by stripping nanoid suffixes
// feat/auth-xY7k → feat/auth
// feat/auth-xY7k-aB2m → feat/auth
// feat/auth-api-k9P2 → feat/auth-api-k9P2 (named suffix, don't strip)
func getStackRoot(branch string) string {
	// If already has stack number suffix, return as-is
	if strings.Contains(branch, "/") {
		parts := strings.Split(branch, "/")
		if len(parts) == 2 {
			suffix := parts[1]
			if regexp.MustCompile(`^d+$`).MatchString(suffix) {
				return branch // Already has stack number
			}
		}
	}

	// For branches with slashes
	if strings.Contains(branch, "/") {
		parts := strings.Split(branch, "/")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]

			// Split by dash and check parts from the end
			suffixParts := strings.Split(suffix, "-")

			// Try to strip up to 2 nanoid suffixes from the end
			stripped := 0
			for i := len(suffixParts) - 1; i >= 0 && stripped < 2; i-- {
				part := suffixParts[i]
				// Check if this part is a 4-char alphanumeric nanoid
				if regexp.MustCompile(`^[a-zA-Z0-9]{4}$`).MatchString(part) {
					if i > 0 {
						// Check if the PREVIOUS part is also a nanoid
						// If yes, we're in a double-nanoid situation, keep stripping
						// If no, check if we have 2+ parts before this (named suffix)
						prevPart := suffixParts[i-1]
						prevIsNanoid := regexp.MustCompile(`^[a-zA-Z0-9]{4}$`).MatchString(prevPart)

						if i >= 2 && !prevIsNanoid {
							// 2+ parts before this and previous is not nanoid = named suffix
							break
						}
						stripped++
					}
				} else {
					break
				}
			}

			if stripped > 0 {
				newSuffix := strings.Join(suffixParts[:len(suffixParts)-stripped], "-")
				return prefix + "/" + newSuffix
			}
		}
	}

	return branch
}

// GenerateStackWindowName generates a window name for a stacked branch
func GenerateStackWindowName(branch string, stackLevel int) string {
	// Get root branch name (strip auto-generated nanoid suffixes)
	root := getStackRoot(branch)

	// Root level (0) has no suffix
	if stackLevel == 0 {
		return GenerateWindowName(root)
	}

	// For stack levels > 0, we need to strip ANY nanoid suffix (including named suffixes)
	// before generating the window name, since the stack level suffix replaces it
	cleanRoot := root
	if strings.Contains(cleanRoot, "/") {
		parts := strings.Split(cleanRoot, "/")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]

			// Strip 4-char alphanumeric suffix from the branch part
			suffixParts := strings.Split(suffix, "-")
			if len(suffixParts) > 1 {
				lastPart := suffixParts[len(suffixParts)-1]
				if regexp.MustCompile(`^[a-zA-Z0-9]{4}$`).MatchString(lastPart) {
					// Remove the last part if it's a 4-char nanoid
					suffix = strings.Join(suffixParts[:len(suffixParts)-1], "-")
					cleanRoot = prefix + "/" + suffix
				}
			}

			// For stack window names, we want to preserve two-word suffixes better than
			// the default GenerateWindowName behavior. Check if suffix has 2-3 words.
			suffixWords := regexp.MustCompile(`[^a-zA-Z0-9]+`).Split(suffix, -1)
			var filteredWords []string
			for _, w := range suffixWords {
				if w != "" {
					filteredWords = append(filteredWords, w)
				}
			}

			// If we have 2-3 words, preserve more of the name
			if len(filteredWords) == 2 || len(filteredWords) == 3 {
				// Keep both words intact for 2-word suffixes
				if len(filteredWords) == 2 {
					abbreviated := suffix
					abbrPrefix := abbreviatePrefix(prefix)
					result := fmt.Sprintf("%s/%s", abbrPrefix, abbreviated)
					// Only truncate if too long
					if len(result) > 20 {
						result = truncate(result, 20)
					}
					return fmt.Sprintf("%s/%d", result, stackLevel)
				}
			}
		}
	}

	baseName := GenerateWindowName(cleanRoot)
	return fmt.Sprintf("%s/%d", baseName, stackLevel)
}

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

// IsInTmux checks if the current process is running inside tmux
func IsInTmux() bool {
	return os.Getenv("TMUX") != ""
}
