package tmux

import (
	"testing"
)

func TestExtractIssueID(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{
			name:   "feature with nova issue ID",
			branch: "feature/nova-123",
			want:   "nova-123",
		},
		{
			name:   "fix with PROJ issue ID",
			branch: "fix/PROJ-456",
			want:   "PROJ-456",
		},
		{
			name:   "bugfix with uppercase issue ID",
			branch: "bugfix/ABC-789",
			want:   "ABC-789",
		},
		{
			name:   "branch without issue ID",
			branch: "feat/auth",
			want:   "",
		},
		{
			name:   "branch without slash",
			branch: "main",
			want:   "",
		},
		{
			name:   "issue ID without dash",
			branch: "feature/nova123",
			want:   "",
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
			got := GenerateWindowName(tt.branch)
			if got != tt.want {
				t.Errorf("GenerateWindowName(%q) = %q, want %q", tt.branch, got, tt.want)
			}
		})
	}
}

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
			got := GenerateStackWindowName(tt.branch, tt.stackLevel)
			if got != tt.want {
				t.Errorf("GenerateStackWindowName(%q, %d) = %q, want %q",
					tt.branch, tt.stackLevel, got, tt.want)
			}
		})
	}
}
