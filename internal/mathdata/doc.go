// Package mathdata provides immutable, generated mathematical metadata shared
// by Untie's TeX and Typst frontends and by the font layer.
//
// The package deliberately exposes lookups rather than its backing slices.
// Callers receive value copies and can therefore neither reorder generated
// data nor mutate process-global tables.
//
// All data is checked in. Upstream KaTeX, Typst, and Typlet sources are needed
// only for an explicit maintainer regeneration, never at build or runtime.
package mathdata
