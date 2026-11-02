// Package layout turns the neutral math IR into a deterministic display list.
//
// It deliberately knows nothing about TeX, Typst, font file formats, or UI
// toolkits. Font data enters through the narrow ports in source.go. All
// intermediate geometry is fixed-point in ems; only flatten.go converts to the
// public float32 display contract.
package layout
