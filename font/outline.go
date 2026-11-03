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
	maxDesignCoordinate = 1 << 30
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
	face := f.newTextFace()
	var totalSegments uint64
	for i := 0; i < glyphCount; i++ {
		gid := tsfont.GID(i)
		advance := face.HorizontalAdvance(gid)
		advanceUnits, err := designCoordinate(advance)
		if err != nil {
			return fmt.Errorf("face %d (%q): glyph %d advance: %w", f.data.Index, f.data.ID, i, err)
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
		bounds, err := outlineBounds(outline.Segments)
		if err != nil {
			return fmt.Errorf("face %d (%q): glyph %d outline: %w", f.data.Index, f.data.ID, i, err)
		}
		f.metrics[i] = GlyphMetrics{
			Advance:    advanceUnits,
			Bounds:     bounds,
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
	face := f.newTextFace()
	outline, ok := face.GlyphDataOutline(tsfont.GID(id))
	if !ok {
		return outlineData{}, false
	}
	segments, err := convertSegments(outline.Segments)
	if err != nil {
		return outlineData{}, false
	}
	data := outlineData{
		bounds:   metric.Bounds,
		segments: segments,
	}
	if err := validateOutline(data); err != nil {
		return outlineData{}, false
	}
	actual, _ := f.outlineMu.entries.LoadOrStore(id, data)
	return actual.(outlineData), true
}

func convertSegments(in []tsot.Segment) ([]Segment, error) {
	out := make([]Segment, 0, len(in)+1)
	open := false
	for i, seg := range in {
		args, err := segmentArgs(seg)
		if err != nil {
			return nil, fmt.Errorf("segment %d: %w", i, err)
		}
		switch seg.Op {
		case tsot.SegmentOpMoveTo:
			if open {
				out = append(out, Segment{Op: geometry.SegClose})
			}
			p, err := point(args[0])
			if err != nil {
				return nil, fmt.Errorf("segment %d move: %w", i, err)
			}
			out = append(out, Segment{
				Op:     geometry.SegMoveTo,
				Points: [3]Point{p},
			})
			open = true
		case tsot.SegmentOpLineTo:
			p, err := point(args[0])
			if err != nil {
				return nil, fmt.Errorf("segment %d line: %w", i, err)
			}
			out = append(out, Segment{
				Op:     geometry.SegLineTo,
				Points: [3]Point{p},
			})
		case tsot.SegmentOpQuadTo:
			p0, err := point(args[0])
			if err != nil {
				return nil, fmt.Errorf("segment %d quadratic control: %w", i, err)
			}
			p1, err := point(args[1])
			if err != nil {
				return nil, fmt.Errorf("segment %d quadratic endpoint: %w", i, err)
			}
			out = append(out, Segment{
				Op:     geometry.SegQuadTo,
				Points: [3]Point{p0, p1},
			})
		case tsot.SegmentOpCubeTo:
			p0, err := point(args[0])
			if err != nil {
				return nil, fmt.Errorf("segment %d cubic control 1: %w", i, err)
			}
			p1, err := point(args[1])
			if err != nil {
				return nil, fmt.Errorf("segment %d cubic control 2: %w", i, err)
			}
			p2, err := point(args[2])
			if err != nil {
				return nil, fmt.Errorf("segment %d cubic endpoint: %w", i, err)
			}
			out = append(out, Segment{
				Op:     geometry.SegCubeTo,
				Points: [3]Point{p0, p1, p2},
			})
		default:
			return nil, fmt.Errorf("segment %d has invalid operation %d", i, seg.Op)
		}
	}
	if open {
		out = append(out, Segment{Op: geometry.SegClose})
	}
	return out, nil
}

func segmentArgs(seg tsot.Segment) ([]tsot.SegmentPoint, error) {
	switch seg.Op {
	case tsot.SegmentOpMoveTo, tsot.SegmentOpLineTo:
		return seg.Args[:1], nil
	case tsot.SegmentOpQuadTo:
		return seg.Args[:2], nil
	case tsot.SegmentOpCubeTo:
		return seg.Args[:3], nil
	default:
		return nil, fmt.Errorf("invalid operation %d", seg.Op)
	}
}

func point(p tsot.SegmentPoint) (Point, error) {
	// The dependency retains CFF 16.16 coordinates as float32. Quantize once at
	// the integer design-unit API boundary, matching ttf-parser/Typst's
	// truncating float-to-int conversion, and flip to y-down.
	x, err := designCoordinate(p.X)
	if err != nil {
		return Point{}, fmt.Errorf("x coordinate: %w", err)
	}
	y, err := designCoordinate(-p.Y)
	if err != nil {
		return Point{}, fmt.Errorf("y coordinate: %w", err)
	}
	return Point{X: x, Y: y}, nil
}

func designCoordinate(v float32) (int32, error) {
	f := float64(v)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("non-finite coordinate %v", v)
	}
	truncated := math.Trunc(f)
	if truncated < -maxDesignCoordinate || truncated > maxDesignCoordinate {
		return 0, fmt.Errorf("coordinate %v is outside the supported design-unit range", v)
	}
	return int32(truncated), nil
}

func outlineBounds(segments []tsot.Segment) (Bounds, error) {
	var b boundsBuilder
	for i, seg := range segments {
		args, err := segmentArgs(seg)
		if err != nil {
			return Bounds{}, fmt.Errorf("segment %d: %w", i, err)
		}
		for _, raw := range args {
			p, err := point(raw)
			if err != nil {
				return Bounds{}, fmt.Errorf("segment %d: %w", i, err)
			}
			b.add(p.X, p.Y)
		}
	}
	return b.bounds(), nil
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
