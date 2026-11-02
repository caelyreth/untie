// Package source defines source locations, diagnostics, and resource limits.
package source

// Range is a half-open UTF-8 byte range in a source string.
//
// The zero Range denotes the empty range at the start of a source. A detached
// range has Start and End both negative.
type Range struct {
	Start int
	End   int
}

// NewRange returns a source range.
func NewRange(start, end int) Range {
	return Range{Start: start, End: end}
}

// Detached returns a range that is not associated with source text.
func Detached() Range {
	return Range{Start: -1, End: -1}
}

// IsDetached reports whether r is not associated with source text.
func (r Range) IsDetached() bool {
	return r.Start < 0 && r.End < 0
}

// IsValid reports whether r is either detached or a valid half-open range.
func (r Range) IsValid() bool {
	return r.IsDetached() || (r.Start >= 0 && r.End >= r.Start)
}

// IsValidFor reports whether r is detached or lies within a source string of
// sourceLen bytes.
func (r Range) IsValidFor(sourceLen int) bool {
	if sourceLen < 0 {
		return false
	}
	return r.IsDetached() || (r.Start >= 0 && r.End >= r.Start && r.End <= sourceLen)
}

// Len returns the byte length of the range.
func (r Range) Len() int {
	if !r.IsValid() || r.IsDetached() {
		return 0
	}
	return r.End - r.Start
}

// Contains reports whether pos lies inside r. End is exclusive.
func (r Range) Contains(pos int) bool {
	if !r.IsValid() || r.IsDetached() {
		return false
	}
	return pos >= r.Start && pos < r.End
}

// ContainsRange reports whether inner lies entirely within r.
func (r Range) ContainsRange(inner Range) bool {
	if inner.IsDetached() {
		return true
	}
	if !r.IsValid() || !inner.IsValid() || r.IsDetached() {
		return false
	}
	return inner.Start >= r.Start && inner.End <= r.End
}

// Slice returns the bytes selected by r. Invalid and detached ranges return nil.
func (r Range) Slice(src string) []byte {
	if !r.IsValidFor(len(src)) || r.IsDetached() {
		return nil
	}
	return []byte(src[r.Start:r.End])
}

// Merge returns the smallest range containing both inputs. Detached inputs are
// ignored. If both inputs are detached, the result is detached.
func Merge(a, b Range) Range {
	switch {
	case a.IsDetached():
		return b
	case b.IsDetached():
		return a
	case !a.IsValid():
		return b
	case !b.IsValid():
		return a
	default:
		return Range{Start: min(a.Start, b.Start), End: max(a.End, b.End)}
	}
}
