// Package crosscheck verifies that the TeX and Typst frontends lower the
// pinned paired corpus to equivalent semantic IR and render equal output.
//
// Crosscheck is test infrastructure. It is not imported by the renderer and
// it uses pinned, tracked fixtures only; references/ is never required.
package crosscheck
