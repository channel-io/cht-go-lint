package lint

import "testing"

func TestShouldSkipFile(t *testing.T) {
	tests := []struct {
		name     string
		skip     []string
		relPath  string
		expected bool
	}{
		{
			name:     "bare filename match",
			skip:     []string{"foo.go"},
			relPath:  "internal/domain/foo.go",
			expected: true,
		},
		{
			name:     "bare filename no match",
			skip:     []string{"bar.go"},
			relPath:  "internal/domain/foo.go",
			expected: false,
		},
		{
			name:     "path suffix match",
			skip:     []string{"encrypt/token_encryptor.go"},
			relPath:  "internal/domain/extension/subdomain/oauth/encrypt/token_encryptor.go",
			expected: true,
		},
		{
			name:     "path suffix no match",
			skip:     []string{"other/token_encryptor.go"},
			relPath:  "internal/domain/extension/subdomain/oauth/encrypt/token_encryptor.go",
			expected: false,
		},
		{
			name:     "glob match",
			skip:     []string{"internal/*/dto.go"},
			relPath:  "internal/app/dto.go",
			expected: true,
		},
		{
			name:     "glob no match deeper path",
			skip:     []string{"internal/*/dto.go"},
			relPath:  "internal/app/sub/dto.go",
			expected: false,
		},
		{
			name:     "bare filename matches basename even with slash patterns present",
			skip:     []string{"encrypt/token_encryptor.go", "config.go"},
			relPath:  "pkg/config.go",
			expected: true,
		},
		{
			name:     "empty skip list",
			skip:     nil,
			relPath:  "anything.go",
			expected: false,
		},
		{
			name:     "exact relative path via suffix",
			skip:     []string{"api/http/middleware/language.go"},
			relPath:  "api/http/middleware/language.go",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := map[string]any{}
			if tt.skip != nil {
				asAny := make([]any, len(tt.skip))
				for i, s := range tt.skip {
					asAny[i] = s
				}
				raw["skip_files"] = asAny
			}
			opts := NewOptions(raw)
			got := opts.ShouldSkipFile(tt.relPath)
			if got != tt.expected {
				t.Errorf("ShouldSkipFile(%q) = %v, want %v (skip=%v)", tt.relPath, got, tt.expected, tt.skip)
			}
		})
	}
}
