package untie

import (
	"container/list"
	"context"
	"sync"

	"github.com/caelyreth/untie/source"
)

const (
	layoutEngineCacheCapacity = 8
	// Increment when the cached Engine implementation changes behavior without
	// a corresponding layout revision bump.
	layoutEngineCacheSchema = 7
)

// layoutEngineCacheKey identifies an engine's immutable construction inputs.
// Style and Features are intentionally absent: cached Engines are
// request-agnostic, while layout results are never cached. Every layout call
// snapshots its Style.Features before entering the engine.
type layoutEngineCacheKey struct {
	fontRevision [32]byte
	limits       source.Limits
	version      string
	schema       uint16
}

func makeLayoutEngineCacheKey(fonts *FontSet, limits source.Limits, version string) layoutEngineCacheKey {
	var revision [32]byte
	if fonts != nil {
		revision = fonts.KeySHA256()
	}
	return layoutEngineCacheKey{
		fontRevision: revision,
		limits:       limits.WithDefaults(),
		version:      version,
		schema:       layoutEngineCacheSchema,
	}
}

type layoutEngineCacheEntry struct {
	key    layoutEngineCacheKey
	engine *Engine
	order  *list.Element
}

type layoutEngineBuild struct {
	done       chan struct{}
	generation uint64
	engine     *Engine
	err        error
}

// layoutEngineCache is a bounded LRU. It retains at most capacity FontSets,
// including their font bytes, so process-wide convenience calls cannot leak an
// unbounded number of prepared configurations. In-flight construction is
// shared between concurrent callers and is not counted against the bound until
// it succeeds.
type layoutEngineCache struct {
	mu         sync.Mutex
	capacity   int
	order      list.List
	entries    map[layoutEngineCacheKey]*layoutEngineCacheEntry
	builds     map[layoutEngineCacheKey]*layoutEngineBuild
	generation uint64
	build      func(*FontSet, source.Limits) (*Engine, error)
}

func newLayoutEngineCache(capacity int) *layoutEngineCache {
	if capacity <= 0 {
		capacity = 1
	}
	return &layoutEngineCache{
		capacity: capacity,
		entries:  make(map[layoutEngineCacheKey]*layoutEngineCacheEntry, capacity),
		builds:   make(map[layoutEngineCacheKey]*layoutEngineBuild),
		build:    NewEngine,
	}
}

var defaultLayoutEngineCache = newLayoutEngineCache(layoutEngineCacheCapacity)

func (c *layoutEngineCache) get(
	ctx context.Context,
	fonts *FontSet,
	limits source.Limits,
) (*Engine, error) {
	if c == nil {
		return nil, ErrInvalidEngine
	}
	if ctx == nil {
		return nil, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := makeLayoutEngineCacheKey(fonts, limits, Version())

	c.mu.Lock()
	if entry := c.entries[key]; entry != nil {
		c.order.MoveToFront(entry.order)
		engine := entry.engine
		c.mu.Unlock()
		return engine, nil
	}
	if build := c.builds[key]; build != nil {
		c.mu.Unlock()
		select {
		case <-build.done:
			return build.engine, build.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	build := &layoutEngineBuild{done: make(chan struct{}), generation: c.generation}
	c.builds[key] = build
	c.mu.Unlock()

	buildFn := c.build
	if buildFn == nil {
		buildFn = NewEngine
	}
	prepared, err := buildFn(fonts, limits)

	c.mu.Lock()
	build.engine = prepared
	build.err = err
	delete(c.builds, key)
	if err == nil && build.generation == c.generation {
		c.insert(key, prepared)
	}
	close(build.done)
	c.mu.Unlock()

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	return prepared, err
}

func (c *layoutEngineCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	c.order.Init()
	clear(c.entries)
}

func (c *layoutEngineCache) insert(key layoutEngineCacheKey, engine *Engine) {
	entry := &layoutEngineCacheEntry{key: key, engine: engine}
	entry.order = c.order.PushFront(entry)
	c.entries[key] = entry
	for len(c.entries) > c.capacity {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		evicted := oldest.Value.(*layoutEngineCacheEntry)
		c.order.Remove(oldest)
		delete(c.entries, evicted.key)
	}
}

func (c *layoutEngineCache) length() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
