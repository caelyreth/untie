// Package untiemygo integrates Untie with MyGo's UI painter and Markdown math
// hooks.
//
// Renderer is a reusable, concurrency-safe adapter. It renders with one
// selected dialect, caches immutable laid-out boxes in a bounded LRU, and
// resolves Untie paint roles through the active MyGo theme at draw time.
// Element turns a formula string directly into a MyGo ui.Element; BindMarkdown
// installs inline and display math callbacks for Markdown views.
//
// Callers may inspect boxes returned by Render, but must treat them as
// immutable. Drawing or caching a box must never modify it or its slices.
package untiemygo
