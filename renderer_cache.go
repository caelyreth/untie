package untie

import (
	"container/list"
	"encoding/binary"
	"sort"
	"sync"

	"github.com/caelyreth/untie/source"
)

type renderExprCacheKey struct {
	source  string
	dialect Dialect
	mode    Mode
	limits  source.Limits
	macros  string
}

func makeRenderExprCacheKey(
	src string,
	dialect Dialect,
	mode Mode,
	opts ParseOptions,
) renderExprCacheKey {
	return renderExprCacheKey{
		source:  src,
		dialect: dialect,
		mode:    mode,
		limits:  opts.Limits.WithDefaults(),
		macros:  renderMacroKey(dialect, opts.Macros),
	}
}

func renderMacroKey(dialect Dialect, macros map[string]Macro) string {
	if dialect != TeX || len(macros) == 0 {
		return ""
	}
	names := make([]string, 0, len(macros))
	for name := range macros {
		names = append(names, name)
	}
	sort.Strings(names)
	size := 0
	for _, name := range names {
		size += 24 + len(name) + len(macros[name].Body)
	}
	encoded := make([]byte, 0, size)
	var scratch [8]byte
	appendString := func(value string) {
		binary.LittleEndian.PutUint64(scratch[:], uint64(len(value)))
		encoded = append(encoded, scratch[:]...)
		encoded = append(encoded, value...)
	}
	for _, name := range names {
		macro := macros[name]
		appendString(name)
		appendString(macro.Body)
		binary.LittleEndian.PutUint64(scratch[:], uint64(macro.Args))
		encoded = append(encoded, scratch[:]...)
	}
	return string(encoded)
}

type renderExprCacheEntry struct {
	key   renderExprCacheKey
	expr  *Expr
	order *list.Element
}

// renderExprCache is a bounded LRU of immutable parsed expressions.
type renderExprCache struct {
	mu       sync.Mutex
	capacity int
	order    list.List
	entries  map[renderExprCacheKey]*renderExprCacheEntry
}

func newRenderExprCache(capacity int) *renderExprCache {
	if capacity <= 0 {
		return nil
	}
	return &renderExprCache{
		capacity: capacity,
		entries:  make(map[renderExprCacheKey]*renderExprCacheEntry, capacity),
	}
}

func (c *renderExprCache) get(key renderExprCacheKey) (*Expr, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[key]
	if entry == nil {
		return nil, false
	}
	c.order.MoveToFront(entry.order)
	return entry.expr, true
}

func (c *renderExprCache) put(key renderExprCacheKey, expr *Expr) {
	if c == nil || expr == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry := c.entries[key]; entry != nil {
		entry.expr = expr
		c.order.MoveToFront(entry.order)
		return
	}
	entry := &renderExprCacheEntry{key: key, expr: expr}
	entry.order = c.order.PushFront(entry)
	c.entries[key] = entry
	for len(c.entries) > c.capacity {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		evicted := oldest.Value.(*renderExprCacheEntry)
		c.order.Remove(oldest)
		delete(c.entries, evicted.key)
	}
}

func (c *renderExprCache) length() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
