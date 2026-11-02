// Package alt derives deterministic plain text and speech from Untie's neutral
// math IR. It never reads source text, rendering geometry, fonts, or layout
// fragments.
//
// Text uses a compact linear notation. Semantic roles distinguish quotients
// from binomials and stacks, matrices from arrays/cases/vectors, and ordinary
// scripts from limits, overlays, and brace labels. Speech spells operators and
// structure as words. Unsupported representations are emitted as visible
// bracket markers. Failures return a deterministic, validated fallback bounded
// by the configured MaxTextBytes.
package alt
