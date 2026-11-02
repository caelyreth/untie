// Package conformance reports frontend behavior against pinned formula corpora.
//
// The package is test infrastructure, not part of the rendering pipeline. It
// evaluates each case under default and explicit resource limits, validates the
// resulting IR, and records a stable disposition for every input. The reporting
// model is dialect-neutral so a future Typst frontend can reuse it without
// sharing parser or layout code.
package conformance
