package engine

import (
	"context"
	"fmt"
	"math"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
)

type assemblyMeasure struct {
	full     int64
	growable int64
	count    int
}

func (a *adapter) assembleGlyph(
	ctx context.Context,
	base layout.Glyph,
	mapped font.Glyph,
	construction font.MathConstruction,
	target int32,
	targetEm geometry.Em,
	axis font.MathAxis,
	text string,
) (layout.Glyph, error) {
	assembly := construction.Assembly
	if assembly == nil || len(assembly.Parts) == 0 {
		return layout.Glyph{}, fmt.Errorf("%w: empty assembly", layout.ErrInsufficientStretch)
	}
	info, ok := a.fonts.Face(mapped.Face)
	if !ok {
		return layout.Glyph{}, fmt.Errorf("%w: assembly face", layout.ErrMissingGlyph)
	}
	if assembly.MinConnectorOverlap < 0 {
		return layout.Glyph{}, fmt.Errorf("%w: negative minimum connector overlap", layout.ErrInvalidInput)
	}
	parts := assembly.Parts
	maxRepeats := a.limits.MaxAssemblyRepeats
	measure := func(repeat int) (assemblyMeasure, error) {
		return a.measureAssembly(ctx, parts, repeat, int64(assembly.MinConnectorOverlap))
	}

	repeat := 0
	current, err := measure(0)
	if err != nil {
		return layout.Glyph{}, err
	}
	if !assemblyReaches(current, int64(target)) {
		one, err := measure(1)
		if err != nil {
			return layout.Glyph{}, err
		}
		perRepeat := int64(0)
		if maxRepeats > 1 {
			two, err := measure(2)
			if err != nil {
				return layout.Glyph{}, err
			}
			perRepeat = (two.full + two.growable) - (one.full + one.growable)
		}
		if perRepeat <= 0 {
			repeat = maxRepeats
		} else {
			needed := int64(1) + ceilDiv64(int64(target)-(one.full+one.growable), perRepeat)
			if needed < 1 {
				needed = 1
			}
			if needed > int64(maxRepeats) {
				needed = int64(maxRepeats)
			}
			repeat = int(needed)
		}
		for repeat > 0 {
			previous, err := measure(repeat - 1)
			if err != nil {
				return layout.Glyph{}, err
			}
			if !assemblyReaches(previous, int64(target)) {
				break
			}
			repeat--
		}
		current, err = measure(repeat)
		if err != nil {
			return layout.Glyph{}, err
		}
		for !assemblyReaches(current, int64(target)) && repeat < maxRepeats {
			repeat++
			current, err = measure(repeat)
			if err != nil {
				return layout.Glyph{}, err
			}
		}
	}
	if !assemblyReaches(current, int64(target)) {
		if repeat >= maxRepeats {
			return layout.Glyph{}, fmt.Errorf(
				"%w: assembly reached repeat limit %d before target %d",
				layout.ErrLimit,
				maxRepeats,
				target,
			)
		}
		return layout.Glyph{}, fmt.Errorf(
			"%w: assembly did not reach %d after %d repeats",
			layout.ErrInsufficientStretch,
			target,
			repeat,
		)
	}

	sequence, err := expandAssembly(parts, repeat)
	if err != nil {
		return layout.Glyph{}, err
	}
	if len(sequence) > a.limits.MaxOutputItems {
		return layout.Glyph{}, fmt.Errorf("%w: assembly part count %d", layout.ErrLimit, len(sequence))
	}
	advances := make([]int64, len(sequence))
	overlaps := make([]int64, max(0, len(sequence)-1))
	for i, part := range sequence {
		if err := checkEvery(ctx, i, 16); err != nil {
			return layout.Glyph{}, err
		}
		if part.FullAdvance < 0 {
			return layout.Glyph{}, fmt.Errorf("%w: negative assembly part advance", layout.ErrInvalidInput)
		}
		advances[i] = int64(part.FullAdvance)
		if i+1 < len(sequence) {
			overlap, err := connectorOverlap(part, sequence[i+1])
			if err != nil {
				return layout.Glyph{}, err
			}
			if overlap < int64(assembly.MinConnectorOverlap) {
				return layout.Glyph{}, fmt.Errorf(
					"%w: connector overlap %d is below minimum %d",
					layout.ErrInvalidInput,
					overlap,
					assembly.MinConnectorOverlap,
				)
			}
			overlaps[i] = overlap
			advances[i] -= overlap
		}
		if advances[i] < 0 {
			return layout.Glyph{}, fmt.Errorf("%w: negative assembly advance", layout.ErrInvalidInput)
		}
	}
	spread := int64(target) - current.full
	if spread < 0 {
		spread = 0
	}
	if spread > current.growable {
		spread = current.growable
	}
	remaining := spread
	for i, overlap := range overlaps {
		capacity := overlap - int64(assembly.MinConnectorOverlap)
		if capacity <= 0 || current.growable == 0 {
			continue
		}
		share := capacity * spread / current.growable
		if share > remaining {
			share = remaining
		}
		advances[i] += share
		remaining -= share
	}
	for remaining > 0 {
		for i, overlap := range overlaps {
			capacity := overlap - int64(assembly.MinConnectorOverlap)
			used := advances[i] - (int64(sequence[i].FullAdvance) - overlap)
			if used < capacity && remaining > 0 {
				advances[i]++
				remaining--
			}
		}
	}

	outlines := make([]font.Outline, len(sequence))
	totalSegments := 0
	for i, part := range sequence {
		if err := checkEvery(ctx, i, 16); err != nil {
			return layout.Glyph{}, err
		}
		outline, ok := a.fonts.Outline(font.Glyph{Face: mapped.Face, ID: part.Glyph})
		if !ok {
			return layout.Glyph{}, fmt.Errorf("%w: assembly part %d", layout.ErrMissingGlyph, part.Glyph)
		}
		if err := reserveSegments(&totalSegments, len(outline.Segments), a.limits.MaxPathSegments); err != nil {
			return layout.Glyph{}, err
		}
		outlines[i] = outline
	}

	combined := make([]font.Segment, 0, totalSegments)
	var cursor int64
	var minX, minY, maxX, maxY int32
	havePoint := false
	var maxVerticalAdvance int64
	for i, part := range sequence {
		if err := checkEvery(ctx, i, 16); err != nil {
			return layout.Glyph{}, err
		}
		outline := outlines[i]
		dx, dy := int64(0), int64(0)
		if axis == font.HorizontalAxis {
			dx = cursor
		} else {
			dy = -cursor - int64(outline.Bounds.YMax)
		}
		for _, segment := range outline.Segments {
			out := segment
			for j := range out.Points {
				x, err := checkedAdd32(out.Points[j].X, dx)
				if err != nil {
					return layout.Glyph{}, err
				}
				y, err := checkedAdd32(out.Points[j].Y, dy)
				if err != nil {
					return layout.Glyph{}, err
				}
				out.Points[j].X = x
				out.Points[j].Y = y
				if !havePoint {
					minX, maxX, minY, maxY = x, x, y, y
					havePoint = true
				} else {
					minX = min32(minX, x)
					maxX = max32(maxX, x)
					minY = min32(minY, y)
					maxY = max32(maxY, y)
				}
			}
			combined = append(combined, out)
		}
		advance, err := checkedAdd64(cursor, advances[i])
		if err != nil {
			return layout.Glyph{}, err
		}
		cursor = advance
		if axis == font.VerticalAxis {
			metrics, ok := a.fonts.GlyphMetrics(font.Glyph{Face: mapped.Face, ID: part.Glyph})
			if !ok {
				return layout.Glyph{}, fmt.Errorf("%w: assembly metrics", layout.ErrMissingGlyph)
			}
			maxVerticalAdvance = max64(maxVerticalAdvance, int64(metrics.Advance))
		}
	}
	if !havePoint {
		return layout.Glyph{}, fmt.Errorf("%w: empty assembly outline", layout.ErrInvalidInput)
	}
	path, err := a.designPath(combined, info.UnitsPerEm)
	if err != nil {
		return layout.Glyph{}, err
	}
	result := base
	result.FontID = info.ID
	result.ID = uint16(mapped.ID)
	result.Text = text
	result.Outline = path
	result.Italic, err = a.emFromUnits(int64(assembly.ItalicCorrection), info.UnitsPerEm)
	if err != nil {
		return layout.Glyph{}, err
	}
	result.AdvanceIncludesItalic = false
	result.Kerns = layout.MathKernInfo{}
	result.TopAccent = 0
	result.HasTopAccent = false
	result.BottomAccent = 0
	result.HasBottomAccent = false
	result.InkBottom = 0
	result.HasInkBottom = false
	result.TextLike = false
	ascent, err := a.emFromUnits(-int64(minY), info.UnitsPerEm)
	if err != nil {
		return layout.Glyph{}, err
	}
	descent := geometry.Em(0)
	if maxY > 0 {
		descent, err = a.emFromUnits(int64(maxY), info.UnitsPerEm)
		if err != nil {
			return layout.Glyph{}, err
		}
	}
	result.Ascent = ascent
	result.Descent = descent
	switch axis {
	case font.HorizontalAxis:
		result.Advance, err = a.emFromUnits(cursor, info.UnitsPerEm)
		if err != nil {
			return layout.Glyph{}, err
		}
		result.TopAccent = result.Advance / 2
		result.HasTopAccent = true
	case font.VerticalAxis:
		result.Advance, err = a.emFromUnits(maxVerticalAdvance, info.UnitsPerEm)
		if err != nil {
			return layout.Glyph{}, err
		}
		// Vertical assembly extent is defined by the spread part advances, not
		// by the union of part ink. Typst sets the assembled frame height to the
		// resolved target before centering it on the math axis.
		result.Ascent = targetEm
		result.Descent = 0
	}
	return result, nil
}

func (a *adapter) measureAssembly(
	ctx context.Context,
	parts []font.AssemblyPart,
	repeat int,
	minOverlap int64,
) (assemblyMeasure, error) {
	if repeat < 0 {
		return assemblyMeasure{}, fmt.Errorf("%w: negative assembly repeat", layout.ErrInvalidInput)
	}
	var measure assemblyMeasure
	var previous *font.AssemblyPart
	for i := range parts {
		part := &parts[i]
		if part.FullAdvance < 0 || part.StartConnector < 0 || part.EndConnector < 0 {
			return assemblyMeasure{}, fmt.Errorf("%w: malformed assembly part", layout.ErrInvalidInput)
		}
		count := 1
		if part.Extender {
			count = repeat
		}
		for j := 0; j < count; j++ {
			if measure.count%64 == 0 {
				if err := ctx.Err(); err != nil {
					return assemblyMeasure{}, err
				}
			}
			if measure.count >= a.limits.MaxOutputItems {
				return assemblyMeasure{}, fmt.Errorf("%w: assembly part count", layout.ErrLimit)
			}
			if previous != nil {
				overlap, err := connectorOverlap(*previous, *part)
				if err != nil {
					return assemblyMeasure{}, err
				}
				if overlap < minOverlap {
					return assemblyMeasure{}, fmt.Errorf(
						"%w: connector overlap %d is below minimum %d",
						layout.ErrInvalidInput,
						overlap,
						minOverlap,
					)
				}
				measure.full, err = checkedAdd64(measure.full, int64(previous.FullAdvance)-overlap)
				if err != nil {
					return assemblyMeasure{}, err
				}
				measure.growable, err = checkedAdd64(measure.growable, overlap-minOverlap)
				if err != nil {
					return assemblyMeasure{}, err
				}
			}
			previous = part
			measure.count++
		}
	}
	if previous != nil {
		var err error
		measure.full, err = checkedAdd64(measure.full, int64(previous.FullAdvance))
		if err != nil {
			return assemblyMeasure{}, err
		}
	}
	return measure, nil
}

func assemblyReaches(measure assemblyMeasure, target int64) bool {
	full := measure.full
	if full < target {
		growth := target - full
		if growth > measure.growable {
			growth = measure.growable
		}
		full += growth
	}
	return target <= full
}

func expandAssembly(parts []font.AssemblyPart, repeat int) ([]font.AssemblyPart, error) {
	count := 0
	for _, part := range parts {
		add := 1
		if part.Extender {
			add = repeat
		}
		if add < 0 || count > math.MaxInt-add {
			return nil, fmt.Errorf("%w: assembly size overflow", layout.ErrLimit)
		}
		count += add
	}
	sequence := make([]font.AssemblyPart, 0, count)
	for _, part := range parts {
		count := 1
		if part.Extender {
			count = repeat
		}
		for i := 0; i < count; i++ {
			sequence = append(sequence, part)
		}
	}
	return sequence, nil
}

func connectorOverlap(left, right font.AssemblyPart) (int64, error) {
	if left.EndConnector < 0 || right.StartConnector < 0 {
		return 0, fmt.Errorf("%w: negative assembly connector", layout.ErrInvalidInput)
	}
	return int64(min32(left.EndConnector, right.StartConnector)), nil
}

func ceilDiv64(a, b int64) int64 {
	if a <= 0 {
		return 0
	}
	if b <= 0 {
		return math.MaxInt64
	}
	return 1 + (a-1)/b
}

func checkedAdd64(a, b int64) (int64, error) {
	if b > 0 && a > math.MaxInt64-b || b < 0 && a < math.MinInt64-b {
		return 0, geometry.ErrOverflow
	}
	return a + b, nil
}

func checkedAdd32(a int32, b int64) (int32, error) {
	value, err := checkedAdd64(int64(a), b)
	if err != nil {
		return 0, err
	}
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, geometry.ErrOverflow
	}
	return int32(value), nil
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
