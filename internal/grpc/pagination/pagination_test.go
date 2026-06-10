package pagination

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPage(t *testing.T) {
	tests := map[string]struct {
		budget   int
		sizes    []int
		wantAdds []bool
		wantStop bool
	}{
		"packs until budget":               {budget: 10, sizes: []int{4, 4, 4}, wantAdds: []bool{true, true, false}, wantStop: true},
		"always accepts first":             {budget: 10, sizes: []int{1000, 1}, wantAdds: []bool{true, false}, wantStop: true},
		"not stopped when everything fits": {budget: 10, sizes: []int{3, 3}, wantAdds: []bool{true, true}, wantStop: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p := New(tt.budget)
			got := make([]bool, 0, len(tt.sizes))
			for _, s := range tt.sizes {
				got = append(got, p.Add(s))
			}
			require.Equal(t, tt.wantAdds, got)
			require.Equal(t, tt.wantStop, p.Stopped())
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := map[string]struct {
		s      string
		marker string
		n      int
		want   string
	}{
		"under limit unchanged": {s: "hello", marker: "...", n: 10, want: "hello"},
		"exact limit unchanged": {s: "hello", marker: "...", n: 5, want: "hello"},
		"clipped with marker":   {s: "hello world", marker: "...", n: 8, want: "hello..."},
		// Each "é" is two bytes; cutting at an odd byte must back up to a boundary.
		"keeps rune boundary": {s: strings.Repeat("é", 10), marker: "", n: 5, want: "éé"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := Truncate(tt.s, tt.marker, tt.n)
			require.Equal(t, tt.want, got)
			require.LessOrEqual(t, len(got), tt.n)
		})
	}
}
