// Package untiemygo integrates Untie with MyGo's UI painter and Markdown math
// hooks.
//
// Renderer is a reusable, concurrency-safe adapter. It renders with one
// selected dialect and caches immutable laid-out boxes in a bounded LRU.
// Element turns a formula string directly into a MyGo ui.Element; BindMarkdown
// installs inline and display math callbacks for Markdown views. DisplayMath
// is the compact fit-to-viewport path, while DisplayMathWidth lays out against
// a known width and wraps before drawing at natural scale. NewStreamingMath
// adapts Untie's coalesced stream sessions to a naturally sized, animated
// display element.
//
// Standalone elements expose semantic alternative text as their accessibility
// label and exact formula source as their value. ElementWithOptions adds
// source-hit callbacks, diagnostic source-range underlines and tooltips, and
// explicit source/plain-text clipboard actions without placing MyGo UI values
// in Untie's immutable display list.
//
// HitAt performs the same inverse viewport transform used by drawing. Inline
// Markdown objects can use nominal baseline metrics while retaining certified
// overflow ink inside the surrounding text clip.
//
// DrawBox draws an existing Untie box without requiring a Renderer. Paint
// resolution happens at draw time through PaintResolver, so cached geometry
// can follow theme changes or a host's own color policy. The default resolver
// preserves explicit formula colors and maps semantic roles through MyGo's
// active theme.
//
// Callers may inspect boxes returned by Render, but must treat them as
// immutable. Drawing or caching a box must never modify it or its slices.
package untiemygo
