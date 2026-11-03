// Package untie renders TeX and Typst math into a UI-independent display list
// using host-supplied OpenType MATH fonts.
//
// Parse and ParseTypst produce immutable expressions. NewEngine prepares a
// caller-owned, concurrency-safe layout environment. NewRenderer is the
// recommended host facade: it combines parsing, layout, and optional bounded
// expression caching while retaining one prepared engine. Package-level Layout
// and LayoutContext use a bounded process-wide engine cache.
//
// Layout output is a flattened display.Box containing glyph outlines, rules,
// source hits, and deterministic diagnostics. The render package provides a
// framework-neutral drawing protocol and fitting helpers. The optional
// encode/svg and encode/raster packages serialize the same display contract
// without coupling the core to a UI framework.
package untie
