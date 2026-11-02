// Package fixture decodes and validates the immutable upstream fixtures used by
// Untie's conformance tests.
//
// This package is intentionally outside the renderer's runtime pipeline. It
// decodes Typlet's compact CST and diagnostics, then adapts fixture-only
// encodings, such as UTF-16 offsets and points, at one boundary so parser, IR,
// layout, and output packages never inherit them.
package fixture
