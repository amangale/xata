// Package pagination splits a response into pages bounded by total byte size,
// so a gRPC reply stays under the receive message limit.
package pagination

import "unicode/utf8"

// Page admits items until their total size exceeds a budget. The caller
// supplies each item's size, so it works for any content. Construct with New.
type Page struct {
	budget  int
	used    int
	count   int
	stopped bool
}

func New(budget int) *Page {
	return &Page{budget: budget}
}

// Add records an item and reports whether it fits. The first item is always
// accepted, so an oversized item never produces an empty page.
func (p *Page) Add(size int) bool {
	if p.count > 0 && p.used+size > p.budget {
		p.stopped = true
		return false
	}
	p.used += size
	p.count++
	return true
}

// Stopped reports whether any item was rejected — the signal that more remain.
func (p *Page) Stopped() bool {
	return p.stopped
}

// Truncate clips s to at most n bytes on a rune boundary, appending marker when
// it clips.
func Truncate(s, marker string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := max(n-len(marker), 0)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + marker
}
