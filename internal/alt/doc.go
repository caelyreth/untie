// Package alt derives deterministic plain text and speech from Untie's neutral
// math IR. It never reads source text, rendering geometry, fonts, or layout
// fragments.
//
// Text uses a compact linear notation: rows concatenate, fractions use
// parenthesized operands with "/", radicals use the Unicode radical signs,
// scripts use "_" and "^" (or Unicode superscripts when available), tables use
// matrix(row; row), and multiline expressions use multiline(row; row). Speech
// spells operators and structure as words. Unsupported representations are
// emitted as visible bracket markers.
package alt
