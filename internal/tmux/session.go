// Package tmux provides a client wrapper for tmux session and window operations.
// It handles session creation, attachment, window management with smart naming,
// and collision-resistant window names using branch name hashing.
package tmux

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

// hashBranch generates a deterministic 4-character hex hash from a branch name
// This provides collision resistance for tmux window names
func hashBranch(branch string) string {
	hash := sha256.Sum256([]byte(branch))
	return hex.EncodeToString(hash[:])[:4]
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
// The name includes a 4-char hash suffix for collision resistance
func GenerateWindowName(branch string) string {
	// Generate hash suffix first (always needed)
	hashSuffix := "-" + hashBranch(branch)

	// 1. Try issue ID extraction: feature/nova-123 → nova-123
	if issue := extractIssueID(branch); issue != "" {
		// Truncate to leave room for hash suffix
		return truncate(issue, 11) + hashSuffix
	}

	// 2. Parse branch components
	prefix, suffix, isMultiPartBranch := parseBranchComponents(branch)

	// 3. Abbreviate suffix based on context
	words := filterEmptyStrings(regexp.MustCompile(`[^a-zA-Z0-9]+`).Split(suffix, -1))

	abbreviated := abbreviateWords(words, prefix, isMultiPartBranch)

	// 4. Combine and truncate to leave room for hash suffix (5 chars: -XXXX)
	var result string
	if prefix != "" {
		result = fmt.Sprintf("%s/%s", prefix, abbreviated)
	} else {
		result = abbreviated
	}

	return truncate(result, 11) + hashSuffix
}

// parseBranchComponents extracts prefix, suffix, and multi-part flag from a branch name
func parseBranchComponents(branch string) (prefix, suffix string, isMultiPartBranch bool) {
	parts := strings.Split(branch, "/")
	if len(parts) >= 2 {
		prefix = abbreviatePrefix(parts[0])
		suffix = parts[len(parts)-1]
		isMultiPartBranch = len(parts) >= 3
	} else {
		suffix = branch
	}
	return prefix, suffix, isMultiPartBranch
}

// filterEmptyStrings removes empty strings from a slice
func filterEmptyStrings(words []string) []string {
	var filtered []string
	for _, word := range words {
		if word != "" {
			filtered = append(filtered, word)
		}
	}
	return filtered
}

// abbreviateWords abbreviates a list of words based on context
func abbreviateWords(words []string, prefix string, isMultiPartBranch bool) string {
	if len(words) == 0 {
		return ""
	}
	if len(words) == 1 {
		return words[0]
	}
	if prefix == "" && len(words) >= 3 {
		return abbreviateThreePlusWordsNoPrefix(words)
	}
	if len(words) == 2 {
		return abbreviateTwoWords(words, isMultiPartBranch)
	}
	return abbreviateMultipleWordsWithPrefix(words)
}

// abbreviateWordOrDigit returns the word as-is if all digits, otherwise first char
func abbreviateWordOrDigit(word string) string {
	if regexp.MustCompile(`^\d+$`).MatchString(word) {
		return word
	}
	if len(word) > 0 {
		return string(word[0])
	}
	return ""
}

// abbreviateTwoWords abbreviates two words based on context
func abbreviateTwoWords(words []string, isMultiPartBranch bool) string {
	first, second := words[0], words[1]

	// If multi-part branch (3+ parts), abbreviate both words aggressively
	if isMultiPartBranch {
		return abbreviateWordOrDigit(first) + "-" + abbreviateWordOrDigit(second)
	}
	if len(first) >= 4 {
		// Keep first intact, abbreviate second
		return first + "-" + abbreviateWordOrDigit(second)
	}
	// First word < 4 chars: abbreviate both
	return abbreviateWordOrDigit(first) + "-" + abbreviateWordOrDigit(second)
}

// abbreviateThreePlusWordsNoPrefix handles 3+ words when there's no prefix
func abbreviateThreePlusWordsNoPrefix(words []string) string {
	result := words[0] + "-" + words[1]
	thirdWord := words[2]
	if len(thirdWord) >= 2 {
		result += "-" + string(thirdWord[0]) + string(thirdWord[1])
	} else {
		result += "-" + thirdWord
	}
	return result
}

// abbreviateMultipleWordsWithPrefix handles 3+ words with a prefix
func abbreviateMultipleWordsWithPrefix(words []string) string {
	result := words[0] + "-" + words[1]
	for i := 2; i < len(words); i++ {
		result += "-" + abbreviateWordOrDigit(words[i])
	}
	return result
}

// isNanoid checks if a string is a 4-char alphanumeric nanoid
func isNanoid(s string) bool {
	return regexp.MustCompile(`^[a-zA-Z0-9]{4}$`).MatchString(s)
}

// getStackRoot returns the root branch name by stripping nanoid suffixes
// feat/auth-xY7k → feat/auth
// feat/auth-xY7k-aB2m → feat/auth
// feat/auth-api-k9P2 → feat/auth-api-k9P2 (named suffix, don't strip)
func getStackRoot(branch string) string {
	prefix, suffix := splitBranchParts(branch)
	if prefix == "" || suffix == "" {
		return branch
	}

	// Check for stack number suffix
	if regexp.MustCompile(`^\d+$`).MatchString(suffix) {
		return branch
	}

	suffixParts := strings.Split(suffix, "-")
	stripped := countNanoidSuffixesToStrip(suffixParts)

	if stripped > 0 {
		newSuffix := strings.Join(suffixParts[:len(suffixParts)-stripped], "-")
		return prefix + "/" + newSuffix
	}

	return branch
}

// splitBranchParts splits a branch into prefix and suffix
func splitBranchParts(branch string) (prefix, suffix string) {
	if !strings.Contains(branch, "/") {
		return "", branch
	}
	parts := strings.Split(branch, "/")
	if len(parts) != 2 {
		return "", branch
	}
	return parts[0], parts[1]
}

// countNanoidSuffixesToStrip counts how many nanoid suffixes can be stripped
func countNanoidSuffixesToStrip(suffixParts []string) int {
	stripped := 0
	for i := len(suffixParts) - 1; i >= 0 && stripped < 2; i-- {
		part := suffixParts[i]
		if !isNanoid(part) {
			break
		}
		if i > 0 {
			prevPart := suffixParts[i-1]
			prevIsNanoid := isNanoid(prevPart)
			if i >= 2 && !prevIsNanoid {
				break
			}
			stripped++
		}
	}
	return stripped
}

// GenerateStackWindowName generates a window name for a stacked branch
func GenerateStackWindowName(branch string, stackLevel int) string {
	root := getStackRoot(branch)

	// Root level (0) has no suffix
	if stackLevel == 0 {
		return GenerateWindowName(root)
	}

	// Try to generate a stack window name with preserved suffix
	if result := tryGenerateStackWindowNameWithSuffix(root, stackLevel); result != "" {
		return result
	}

	baseName := GenerateWindowName(root)
	return fmt.Sprintf("%s/%d", baseName, stackLevel)
}

// tryGenerateStackWindowNameWithSuffix attempts to generate a window name preserving suffix words
func tryGenerateStackWindowNameWithSuffix(root string, stackLevel int) string {
	prefix, suffix := splitBranchParts(root)
	if prefix == "" || suffix == "" {
		return ""
	}

	// Strip 4-char nanoid suffix from the branch part
	cleanSuffix := stripNanoidSuffix(suffix)

	// Check if suffix has 2 words for special handling
	words := filterEmptyStrings(regexp.MustCompile(`[^a-zA-Z0-9]+`).Split(cleanSuffix, -1))
	if len(words) == 2 {
		abbrPrefix := abbreviatePrefix(prefix)
		result := fmt.Sprintf("%s/%s", abbrPrefix, cleanSuffix)
		if len(result) > 20 {
			result = truncate(result, 20)
		}
		return fmt.Sprintf("%s/%d", result, stackLevel)
	}

	return ""
}

// stripNanoidSuffix removes a trailing 4-char nanoid from a suffix
func stripNanoidSuffix(suffix string) string {
	suffixParts := strings.Split(suffix, "-")
	if len(suffixParts) > 1 {
		lastPart := suffixParts[len(suffixParts)-1]
		if isNanoid(lastPart) {
			return strings.Join(suffixParts[:len(suffixParts)-1], "-")
		}
	}
	return suffix
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
