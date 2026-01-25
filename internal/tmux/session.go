package tmux

import (
	"bytes"
	"fmt"
	"os/exec"
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
