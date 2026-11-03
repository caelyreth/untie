package untiemygo

import (
	"container/list"
	"strconv"
	"strings"
	"sync"

	"github.com/caelyreth/untie"
)

type layoutStyleKey struct {
	size        float32
	maxWidth    float32
	align       untie.Align
	scriptLevel int
	features    string
	glyphRefs   bool
	generateAlt bool
	colorPolicy untie.ColorPolicy
}

type layoutKey struct {
	source  string
	dialect untie.Dialect
	mode    untie.Mode
	style   layoutStyleKey
}

func makeLayoutKey(source string, dialect untie.Dialect, mode untie.Mode, style untie.Style) layoutKey {
	return layoutKey{
		source:  source,
		dialect: dialect,
		mode:    mode,
		style: layoutStyleKey{
			size:        style.Size,
			maxWidth:    style.MaxWidth,
			align:       style.Align,
			scriptLevel: style.ScriptLevel,
			features:    styleFeaturesKey(style.Features),
			glyphRefs:   style.GlyphRefs,
			generateAlt: style.GenerateAlt,
			colorPolicy: style.ColorPolicy,
		},
	}
}

func styleFeaturesKey(features []string) string {
	if len(features) == 0 {
		return ""
	}
	var key strings.Builder
	for _, feature := range features {
		key.WriteString(strconv.Itoa(len(feature)))
		key.WriteByte(':')
		key.WriteString(feature)
		key.WriteByte(';')
	}
	return key.String()
}

type cachedRender struct {
	box         *untie.Box
	diagnostics []untie.Diagnostic
}

type cacheEntry struct {
	key   layoutKey
	value cachedRender
	order *list.Element
}

// CacheStats reports bounded layout-cache activity since construction.
type CacheStats struct {
	Entries   int
	Capacity  int
	Hits      int
	Misses    int
	Evictions int
}

type layoutCache struct {
	mu        sync.Mutex
	capacity  int
	order     list.List
	entries   map[layoutKey]*cacheEntry
	hits      int
	misses    int
	evictions int
}

func newLayoutCache(capacity int) *layoutCache {
	if capacity <= 0 {
		return nil
	}
	return &layoutCache{
		capacity: capacity,
		entries:  make(map[layoutKey]*cacheEntry, capacity),
	}
}

func (c *layoutCache) get(key layoutKey) (cachedRender, bool) {
	if c == nil {
		return cachedRender{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[key]
	if entry == nil {
		c.misses++
		return cachedRender{}, false
	}
	c.hits++
	c.order.MoveToFront(entry.order)
	value := entry.value
	value.diagnostics = cloneDiagnostics(value.diagnostics)
	return value, true
}

func (c *layoutCache) put(key layoutKey, value cachedRender) {
	if c == nil || value.box == nil {
		return
	}
	value.diagnostics = cloneDiagnostics(value.diagnostics)
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry := c.entries[key]; entry != nil {
		entry.value = value
		c.order.MoveToFront(entry.order)
		return
	}
	entry := &cacheEntry{key: key, value: value}
	entry.order = c.order.PushFront(entry)
	c.entries[key] = entry
	for len(c.entries) > c.capacity {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		evicted := oldest.Value.(*cacheEntry)
		c.order.Remove(oldest)
		delete(c.entries, evicted.key)
		c.evictions++
	}
}

func cloneDiagnostics(in []untie.Diagnostic) []untie.Diagnostic {
	if len(in) == 0 {
		return nil
	}
	out := make([]untie.Diagnostic, len(in))
	for i, diagnostic := range in {
		out[i] = diagnostic.Clone()
	}
	return out
}

func (c *layoutCache) stats() CacheStats {
	if c == nil {
		return CacheStats{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return CacheStats{
		Entries:   len(c.entries),
		Capacity:  c.capacity,
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
	}
}
