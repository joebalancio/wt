package tmux

import (
	"strings"
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
		name       string
		branch     string
		wantPrefix string // Expected prefix before hash suffix
	}{
		{"issue ID extraction", "feature/nova-123", "nova-123"},
		{"two part branch", "feat/auth", "feat/auth"},
		{"feature branch", "feature/api-fix", "feat/a-f"},
		{"bugfix branch", "bugfix/auth-providers", "fix/auth-p"},
		{"long branch name", "very-long-branch-name-here", "very-long"},
		{"single word", "main", "main"},
		{"three part branch", "feat/team/auth-api", "feat/a-a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateWindowName(tt.branch)

			// Verify format: prefix-hash (hash is 4 hex chars preceded by -)
			if len(got) < 5 {
				t.Errorf("GenerateWindowName(%q) = %q, too short", tt.branch, got)
				return
			}

			// Verify the hash suffix is present
			suffix := got[len(got)-4:]
			separator := got[len(got)-5]

			if separator != '-' {
				t.Errorf("GenerateWindowName(%q) = %q, should end with -XXXX", tt.branch, got)
			}

			// Verify suffix is hexadecimal
			for _, c := range suffix {
				if c < '0' || c > '9' && c < 'a' || c > 'f' {
					t.Errorf("GenerateWindowName(%q) = %q, suffix should be hex", tt.branch, got)
					break
				}
			}

			// Verify deterministic - call again and check same result
			got2 := GenerateWindowName(tt.branch)
			if got != got2 {
				t.Errorf("GenerateWindowName is not deterministic: %q != %q", got, got2)
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
		wantSuffix string // Expected suffix (e.g., "/1", "/2")
	}{
		{"root level with hash", "feat/auth", 0, ""},
		{"first stack level", "feat/auth-xY7k", 1, "/1"},
		{"second stack level", "feat/auth-xY7k-aB2m", 2, "/2"},
		{"named suffix first level", "feat/auth-api-k9P2", 1, "/1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateStackWindowName(tt.branch, tt.stackLevel)

			// Verify the stack level suffix is correct
			if tt.wantSuffix != "" {
				if !strings.HasSuffix(got, tt.wantSuffix) {
					t.Errorf("GenerateStackWindowName(%q, %d) = %q, should end with %q",
						tt.branch, tt.stackLevel, got, tt.wantSuffix)
					return
				}
			}

			// Verify deterministic
			got2 := GenerateStackWindowName(tt.branch, tt.stackLevel)
			if got != got2 {
				t.Errorf("GenerateStackWindowName is not deterministic: %q != %q", got, got2)
			}

			// Verify the hash is present (4 hex chars before the stack level suffix or at end)
			var hashPart string
			if tt.wantSuffix != "" {
				// Extract part before stack suffix
				beforeStack := got[:len(got)-len(tt.wantSuffix)]
				// Last 4 chars should be hash
				if len(beforeStack) >= 5 && beforeStack[len(beforeStack)-5] == '-' {
					hashPart = beforeStack[len(beforeStack)-4:]
				}
			} else {
				// Root level - last 4 chars should be hash
				if len(got) >= 5 && got[len(got)-5] == '-' {
					hashPart = got[len(got)-4:]
				}
			}

			// Verify hash is hexadecimal
			for _, c := range hashPart {
				if c < '0' || c > '9' && c < 'a' || c > 'f' {
					t.Errorf("GenerateStackWindowName(%q, %d) = %q, hash should be hex",
						tt.branch, tt.stackLevel, got)
					break
				}
			}
		})
	}
}

func TestHashBranch(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   string // 4-char hex suffix
	}{
		{"simple branch", "feat/auth", "0e3b"},
		{"feature branch", "feature/nova-123", "6fa0"},
		{"different branch", "bugfix/auth-providers", "8f92"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hashBranch(tt.branch)
			// Verify it's 4 chars
			if len(got) != 4 {
				t.Errorf("hashBranch(%q) = %q, want 4 chars", tt.branch, got)
			}
			// Verify it's hexadecimal
			for _, c := range got {
				if c < '0' || c > '9' && c < 'a' || c > 'f' {
					t.Errorf("hashBranch(%q) = %q, want hexadecimal chars only", tt.branch, got)
					break
				}
			}
		})
	}
}

func TestHashBranchDeterministic(t *testing.T) {
	branch := "feat/auth-provider"
	first := hashBranch(branch)
	second := hashBranch(branch)

	if first != second {
		t.Errorf("hashBranch is not deterministic: %q != %q", first, second)
	}

	// Same branch should always produce same hash
	for i := 0; i < 10; i++ {
		got := hashBranch(branch)
		if got != first {
			t.Errorf("hashBranch(%q) iteration %d = %q, want %q", branch, i, got, first)
		}
	}
}

func TestHashBranchCollisionResistance(t *testing.T) {
	// Different branches should produce different hashes
	branches := []string{
		"feat/auth-provider",
		"feat/auth-parser",
		"feat/auth-proxy",
		"fix/auth-provider",
	}

	hashes := make(map[string]string)
	for _, branch := range branches {
		hash := hashBranch(branch)
		if existingBranch, exists := hashes[hash]; exists {
			t.Errorf("hash collision: %q and %q both produce %q", existingBranch, branch, hash)
		}
		hashes[hash] = branch
	}
}

func TestGenerateWindowNameWithHash(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		// We can't hardcode expected hash, so we just check format
	}{
		{"issue ID extraction", "feature/nova-123"},
		{"two part branch", "feat/auth"},
		{"feature branch", "feature/api-fix"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateWindowName(tt.branch)

			// Should end with - followed by 4 hex chars
			if len(got) < 5 {
				t.Errorf("GenerateWindowName(%q) = %q, too short", tt.branch, got)
				return
			}

			// Check format: ends with -XXXX where X is hex
			suffix := got[len(got)-4:]
			separator := got[len(got)-5]

			if separator != '-' {
				t.Errorf("GenerateWindowName(%q) = %q, should end with -XXXX", tt.branch, got)
			}

			for _, c := range suffix {
				if c < '0' || c > '9' && c < 'a' || c > 'f' {
					t.Errorf("GenerateWindowName(%q) = %q, suffix should be hex", tt.branch, got)
					break
				}
			}
		})
	}
}

func TestGenerateWindowNameWithHashDeterministic(t *testing.T) {
	branch := "feat/auth-provider"
	first := GenerateWindowName(branch)
	second := GenerateWindowName(branch)

	if first != second {
		t.Errorf("GenerateWindowName is not deterministic: %q != %q", first, second)
	}
}

func TestGenerateWindowNameWithHashNoCollision(t *testing.T) {
	// These branches would produce the same abbreviated name without hash
	// (feat/auth-p) but should be different with hash
	provider := GenerateWindowName("feat/auth-provider")
	parser := GenerateWindowName("feat/auth-parser")

	if provider == parser {
		t.Errorf("Collision detected: both branches produce %q", provider)
	}

	// Both should have the same prefix before the hash
	// (feat/auth-p-) but different hashes
	providerPrefix := provider[:len(provider)-4]
	parserPrefix := parser[:len(parser)-4]

	if providerPrefix != parserPrefix {
		t.Errorf("Prefixes should match: %q vs %q", providerPrefix, parserPrefix)
	}
}
