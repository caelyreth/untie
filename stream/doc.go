// Package stream renders append-only formula updates as stable display
// transitions.
//
// A Session coalesces rapidly changing source before rendering, retains the
// latest immutable boxes, and exposes a Transition that moves, resizes, fades,
// and scales visible commands without rebuilding its path geometry for every
// animation frame. The package has no UI-framework dependency; hosts walk the
// sampled commands through their own painter.
package stream
