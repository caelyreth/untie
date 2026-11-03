// Package svg encodes a display.Box as a standalone SVG document.
//
// The encoder is an optional pure-Go output adapter. It consumes the visible
// commands exposed by render.Walk, resolves paint roles against a caller
// supplied Palette, and includes certified box overflow in its viewBox.
//
// SVG output is intended for embedding and interchange, not as a stable wire
// protocol. Use an application-owned version if serialized output must be
// persisted across upgrades.
package svg
