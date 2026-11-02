// Package mathdata provides immutable, generated mathematical metadata shared
// by Untie's TeX and Typst frontends and by the font layer.
//
// The package deliberately exposes lookups rather than its backing slices.
// Callers receive value copies and can therefore neither reorder generated
// data nor mutate process-global tables.
//
// All data is checked in. References to KaTeX, Typst, and Typlet are needed
// only when regenerating, never at build or runtime.
//
//go:generate go run ./gen -refs ../../references -out .
package mathdata
