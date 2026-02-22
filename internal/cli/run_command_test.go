package cli

import "testing"

func TestExpandRunTemplate(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		worktreePath string
		want         string
	}{
		{
			name:         "expands worktree_path",
			command:      "cd {worktree_path} && claude",
			worktreePath: "/home/user/worktrees/feat-auth",
			want:         "cd /home/user/worktrees/feat-auth && claude",
		},
		{
			name:         "empty_command",
			command:      "",
			worktreePath: "/path/to/worktree",
			want:         "",
		},
		{
			name:         "no_templates",
			command:      "claude",
			worktreePath: "/path",
			want:         "claude",
		},
		{
			name:         "unknown_template_passthrough",
			command:      "cd {branch} && claude",
			worktreePath: "/path",
			want:         "cd {branch} && claude",
		},
		{
			name:         "multiple_worktree_path",
			command:      "cd {worktree_path} && echo {worktree_path}",
			worktreePath: "/home/user/wt",
			want:         "cd /home/user/wt && echo /home/user/wt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandRunTemplate(tt.command, tt.worktreePath)
			if got != tt.want {
				t.Errorf("expandRunTemplate(%q, %q) = %q, want %q", tt.command, tt.worktreePath, got, tt.want)
			}
		})
	}
}

func TestShouldSkipRun(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		windowExisted bool
		want          bool
	}{
		{
			name:          "empty_command_skips",
			command:       "",
			windowExisted: false,
			want:          true,
		},
		{
			name:          "window_existed_skips",
			command:       "claude",
			windowExisted: true,
			want:          true,
		},
		{
			name:          "new_window_with_command_runs",
			command:       "claude",
			windowExisted: false,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipRun(tt.command, tt.windowExisted)
			if got != tt.want {
				t.Errorf("shouldSkipRun(%q, %v) = %v, want %v", tt.command, tt.windowExisted, got, tt.want)
			}
		})
	}
}

func TestBuildShellCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{
			name:    "simple_command",
			command: "claude",
			want:    []string{"sh", "-c", "claude"},
		},
		{
			name:    "command_with_args",
			command: "claude --prompt 'fix bug'",
			want:    []string{"sh", "-c", "claude --prompt 'fix bug'"},
		},
		{
			name:    "command_with_pipe",
			command: "echo hello | cat",
			want:    []string{"sh", "-c", "echo hello | cat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildShellCommand(tt.command)
			if !equalSlices(got, tt.want) {
				t.Errorf("buildShellCommand(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
