package untiemygo

// ElementOptions configure optional editor integration for one formula
// element.
//
// The zero value preserves the basic accessible element: semantic
// accessibility, visible recovery, safe source fallback, and no interaction
// callbacks or diagnostic decorations.
type ElementOptions struct {
	// OnHit receives hover, press, and click events resolved through Box.At.
	// It runs on the MyGo UI thread. MyGo does not report a reliable hover-exit
	// event, so hosts should treat a new hover or press elsewhere as leaving the
	// previous hit.
	OnHit func(HitEvent)
	// Diagnostics controls optional source-range underlines and a tooltip.
	Diagnostics DiagnosticOptions
	// Clipboard adds a context menu with explicit plain-text and source-copy
	// actions.
	Clipboard ClipboardOptions
}
