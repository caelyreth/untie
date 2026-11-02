package font

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/caelyreth/untie/geometry"
	tsfont "github.com/go-text/typesetting/font"
	tsot "github.com/go-text/typesetting/font/opentype"
)

const (
	maxSegmentsPerGlyph = 1 << 20
	maxSegmentsPerFace  = 16 << 20
)

type outlineCache struct {
	entries sync.Map // map[GlyphID]outlineData
}

type outlineData struct {
	bounds   Bounds
	segments []Segment
}

func (f *parsedFace) readGlyphMetrics() error {
	glyphCount := f.info.GlyphCount
	if glyphCount <= 0 {
		return fmt.Errorf("face %d (%q): font contains no glyphs", f.data.Index, f.data.ID)
	}
	f.metrics = make([]GlyphMetrics, glyphCount)
	face := tsfont.NewFace(f.glyphFont)
	var totalSegments uint64
	for i := 0; i < glyphCount; i++ {
		gid := tsfont.GID(i)
		advance := face.HorizontalAdvance(gid)
		if math.IsNaN(float64(advance)) || math.IsInf(float64(advance), 0) {
			return fmt.Errorf("face %d (%q): glyph %d has non-finite advance", f.data.Index, f.data.ID, i)
		}
		outline, ok := face.GlyphDataOutline(gid)
		if !ok {
			return fmt.Errorf("face %d (%q): glyph %d has no readable outline", f.data.Index, f.data.ID, i)
		}
		if len(outline.Segments) > maxSegmentsPerGlyph {
			return fmt.Errorf("face %d (%q): glyph %d has %d segments; limit is %d", f.data.Index, f.data.ID, i, len(outline.Segments), maxSegmentsPerGlyph)
		}
		totalSegments += uint64(len(outline.Segments))
		if totalSegments > maxSegmentsPerFace {
			return fmt.Errorf("face %d (%q): outlines exceed %d segments", f.data.Index, f.data.ID, maxSegmentsPerFace)
		}
		f.metrics[i] = GlyphMetrics{
			Advance:    int32(advance),
			Bounds:     outlineBounds(outline.Segments),
			HasOutline: len(outline.Segments) != 0,
		}
	}
	return nil
}

func (f *parsedFace) outline(id GlyphID) (outlineData, bool) {
	if int(id) >= len(f.metrics) {
		return outlineData{}, false
	}
	if cached, ok := f.outlineMu.entries.Load(id); ok {
		return cached.(outlineData), true
	}
	metric := f.metrics[id]
	if !metric.HasOutline {
		data := outlineData{bounds: metric.Bounds}
		actual, _ := f.outlineMu.entries.LoadOrStore(id, data)
		return actual.(outlineData), true
	}
	face := tsfont.NewFace(f.glyphFont)
	outline, ok := face.GlyphDataOutline(tsfont.GID(id))
	if !ok {
		return outlineData{}, false
	}
	data := outlineData{
		bounds:   metric.Bounds,
		segments: convertSegments(outline.Segments),
	}
	if err := validateOutline(data); err != nil {
		return outlineData{}, false
	}
	actual, _ := f.outlineMu.entries.LoadOrStore(id, data)
	return actual.(outlineData), true
}

func convertSegments(in []tsot.Segment) []Segment {
	out := make([]Segment, 0, len(in)+1)
	open := false
	for _, seg := range in {
		switch seg.Op {
		case tsot.SegmentOpMoveTo:
			if open {
				out = append(out, Segment{Op: geometry.SegClose})
			}
			out = append(out, Segment{
				Op:     geometry.SegMoveTo,
				Points: [3]Point{point(seg.Args[0])},
			})
			open = true
		case tsot.SegmentOpLineTo:
			out = append(out, Segment{
				Op:     geometry.SegLineTo,
				Points: [3]Point{point(seg.Args[0])},
			})
		case tsot.SegmentOpQuadTo:
			out = append(out, Segment{
				Op:     geometry.SegQuadTo,
				Points: [3]Point{point(seg.Args[0]), point(seg.Args[1])},
			})
		case tsot.SegmentOpCubeTo:
			out = append(out, Segment{
				Op:     geometry.SegCubeTo,
				Points: [3]Point{point(seg.Args[0]), point(seg.Args[1]), point(seg.Args[2])},
			})
		}
	}
	if open {
		out = append(out, Segment{Op: geometry.SegClose})
	}
	return out
}

func point(p tsot.SegmentPoint) Point {
	// The outline parser retains CFF 16.16 coordinates as float32. Convert once
	// to whole design units and flip to the renderer's y-down convention.
	return Point{X: int32(p.X), Y: int32(-p.Y)}
}

func outlineBounds(segments []tsot.Segment) Bounds {
	var b boundsBuilder
	for _, seg := range segments {
		for _, p := range seg.ArgsSlice() {
			b.add(int32(p.X), int32(-p.Y))
		}
	}
	return b.bounds()
}

type boundsBuilder struct {
	minX, minY int32
	maxX, maxY int32
	ok         bool
}

func (b *boundsBuilder) add(x, y int32) {
	if !b.ok {
		b.minX, b.maxX = x, x
		b.minY, b.maxY = y, y
		b.ok = true
		return
	}
	b.minX = min(b.minX, x)
	b.minY = min(b.minY, y)
	b.maxX = max(b.maxX, x)
	b.maxY = max(b.maxY, y)
}

func (b *boundsBuilder) bounds() Bounds {
	if !b.ok {
		return Bounds{}
	}
	return Bounds{XMin: b.minX, YMin: b.minY, XMax: b.maxX, YMax: b.maxY}
}

func validateOutline(data outlineData) error {
	for i, seg := range data.segments {
		switch seg.Op {
		case geometry.SegMoveTo, geometry.SegLineTo, geometry.SegQuadTo, geometry.SegCubeTo, geometry.SegClose:
		default:
			return fmt.Errorf("segment %d has invalid operation %d", i, seg.Op)
		}
	}
	if data.bounds.XMin > data.bounds.XMax || data.bounds.YMin > data.bounds.YMax {
		return errors.New("inverted outline bounds")
	}
	return nil
}
