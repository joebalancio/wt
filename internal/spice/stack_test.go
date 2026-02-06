package spice

import (
	"testing"
)

func TestGetStackLevel(t *testing.T) {
	tests := []struct {
		name    string
		stack   []*Branch
		branch  string
		want    int
		wantErr bool
	}{
		{
			name: "root branch",
			stack: []*Branch{
				{Name: "main", IsRoot: true},
				{Name: "feat/auth", IsRoot: false},
			},
			branch:  "main",
			want:    0,
			wantErr: false,
		},
		{
			name: "first stacked branch",
			stack: []*Branch{
				{Name: "main", IsRoot: true},
				{Name: "feat/auth", IsRoot: false},
			},
			branch:  "feat/auth",
			want:    1,
			wantErr: false,
		},
		{
			name: "branch not in stack",
			stack: []*Branch{
				{Name: "main", IsRoot: true},
				{Name: "feat/other", IsRoot: false},
			},
			branch:  "feat/auth",
			want:    0,
			wantErr: true,
		},
		{
			name:    "empty stack",
			stack:   []*Branch{},
			branch:  "feat/auth",
			want:    0,
			wantErr: true,
		},
		{
			name: "multi-level stack",
			stack: []*Branch{
				{Name: "main", IsRoot: true},
				{Name: "feat/auth", IsRoot: false},
				{Name: "feat/auth-api", IsRoot: false},
			},
			branch:  "feat/auth-api",
			want:    2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{}
			got, err := c.GetStackLevel(tt.stack, tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetStackLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetStackLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
