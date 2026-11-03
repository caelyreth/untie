// Package untie renders TeX and Typst math into a UI-independent display list
// using host-supplied OpenType MATH fonts.
//
// Parse and ParseTypst produce immutable expressions. NewEngine prepares a
// caller-owned, concurrency-safe layout environment; NewRenderer adds a
// one-call parse/layout facade with optional bounded expression caching.
// Package-level Layout and LayoutContext use a bounded process-wide engine
// cache. Layout output is a flattened display.Box containing glyph outlines,
// rules, source hits, and deterministic diagnostics.
package untie
