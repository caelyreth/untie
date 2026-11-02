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
	full     geometry.Em
	growable geometry.Em
	count    int
}

func (a *adapter) assembleGlyph(
	ctx context.Context,
	base layout.Glyph,
	mapped font.Glyph,
	construction font.MathConstruction,
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
	minOverlap, err := a.emFromUnits(int64(assembly.MinConnectorOverlap), info.UnitsPerEm)
	if err != nil {
		return layout.Glyph{}, err
	}
	measure := func(repeat int) (assemblyMeasure, error) {
		return a.measureAssembly(ctx, parts, repeat, minOverlap, info.UnitsPerEm)
	}

	repeat := 0
	current, err := measure(0)
	if err != nil {
		return layout.Glyph{}, err
	}
	if !assemblyReaches(current, targetEm) {
		one, err := measure(1)
		if err != nil {
			return layout.Glyph{}, err
		}
		var perRepeat geometry.Em
		if maxRepeats > 1 {
			two, err := measure(2)
			if err != nil {
				return layout.Glyph{}, err
			}
			twoReach, err := two.full.Add(two.growable)
			if err != nil {
				return layout.Glyph{}, err
			}
			oneReach, err := one.full.Add(one.growable)
			if err != nil {
				return layout.Glyph{}, err
			}
			perRepeat, err = subEm(twoReach, oneReach)
			if err != nil {
				return layout.Glyph{}, err
			}
		}
		if perRepeat <= 0 {
			repeat = maxRepeats
		} else {
			reach, err := one.full.Add(one.growable)
			if err != nil {
				return layout.Glyph{}, err
			}
			deficit, err := subEm(targetEm, reach)
			if err != nil {
				return layout.Glyph{}, err
			}
			needed := int64(1) + ceilDiv64(int64(deficit), int64(perRepeat))
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
			if !assemblyReaches(previous, targetEm) {
				break
			}
			repeat--
		}
		current, err = measure(repeat)
		if err != nil {
			return layout.Glyph{}, err
		}
		for !assemblyReaches(current, targetEm) && repeat < maxRepeats {
			repeat++
			current, err = measure(repeat)
			if err != nil {
				return layout.Glyph{}, err
			}
		}
	}
	if !assemblyReaches(current, targetEm) {
		if repeat >= maxRepeats {
			return layout.Glyph{}, fmt.Errorf(
				"%w: assembly reached repeat limit %d before target %d",
				layout.ErrLimit,
				maxRepeats,
				targetEm,
			)
		}
		return layout.Glyph{}, fmt.Errorf(
			"%w: assembly did not reach %d after %d repeats",
			layout.ErrInsufficientStretch,
			targetEm,
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
	advances := make([]geometry.Em, len(sequence))
	overlaps := make([]geometry.Em, max(0, len(sequence)-1))
	for i, part := range sequence {
		if err := checkEvery(ctx, i, 16); err != nil {
			return layout.Glyph{}, err
		}
		if part.FullAdvance < 0 {
			return layout.Glyph{}, fmt.Errorf("%w: negative assembly part advance", layout.ErrInvalidInput)
		}
		advances[i], err = a.emFromUnits(int64(part.FullAdvance), info.UnitsPerEm)
		if err != nil {
			return layout.Glyph{}, err
		}
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
			overlaps[i], err = a.emFromUnits(overlap, info.UnitsPerEm)
			if err != nil {
				return layout.Glyph{}, err
			}
			advances[i], err = advances[i].Add(-overlaps[i])
			if err != nil {
				return layout.Glyph{}, err
			}
		}
		if advances[i] < 0 {
			return layout.Glyph{}, fmt.Errorf("%w: negative assembly advance", layout.ErrInvalidInput)
		}
	}
	spread, err := subEm(targetEm, current.full)
	if err != nil {
		return layout.Glyph{}, err
	}
	if spread < 0 {
		spread = 0
	}
	if spread > current.growable {
		spread = current.growable
	}
	remaining := spread
	for i, overlap := range overlaps {
		capacity, err := overlap.Add(-minOverlap)
		if err != nil {
			return layout.Glyph{}, err
		}
		if capacity <= 0 || current.growable == 0 {
			continue
		}
		share, err := capacity.MulDiv(int64(spread), int64(current.growable))
		if err != nil {
			return layout.Glyph{}, err
		}
		if share > remaining {
			share = remaining
		}
		advances[i], err = advances[i].Add(share)
		if err != nil {
			return layout.Glyph{}, err
		}
		remaining, err = remaining.Add(-share)
		if err != nil {
			return layout.Glyph{}, err
		}
	}
	for remaining > 0 {
		for i, overlap := range overlaps {
			capacity, err := overlap.Add(-minOverlap)
			if err != nil {
				return layout.Glyph{}, err
			}
			fullAdvance, err := a.emFromUnits(int64(sequence[i].FullAdvance), info.UnitsPerEm)
			if err != nil {
				return layout.Glyph{}, err
			}
			baseAdvance, err := fullAdvance.Add(-overlap)
			if err != nil {
				return layout.Glyph{}, err
			}
			used, err := advances[i].Add(-baseAdvance)
			if err != nil {
				return layout.Glyph{}, err
			}
			if used < capacity && remaining > 0 {
				advances[i]++
				remaining--
			}
		}
	}

	placed := make([]layout.PositionedGlyph, len(sequence))
	var cursor geometry.Em
	var maxAscent, maxDescent geometry.Em
	var maxAdvance geometry.Em
	fullExtent, err := current.full.Add(spread)
	if err != nil {
		return layout.Glyph{}, err
	}
	for i, part := range sequence {
		if err := checkEvery(ctx, i, 16); err != nil {
			return layout.Glyph{}, err
		}
		glyph, err := a.glyphFromFont(
			font.Glyph{Face: mapped.Face, ID: part.Glyph},
			layout.Style{},
			true,
			text,
		)
		if err != nil {
			return layout.Glyph{}, fmt.Errorf("%w: assembly part %d", err, part.Glyph)
		}
		var at layout.Point
		switch axis {
		case font.HorizontalAxis:
			at.X = cursor
			maxAscent = max(maxAscent, glyph.Ascent)
			maxDescent = max(maxDescent, glyph.Descent)
		case font.VerticalAxis:
			half, err := fullExtent.MulDiv(1, 2)
			if err != nil {
				return layout.Glyph{}, err
			}
			top, err := half.Add(-a.metrics.AxisHeight)
			if err != nil {
				return layout.Glyph{}, err
			}
			top, err = top.Add(-cursor)
			if err != nil {
				return layout.Glyph{}, err
			}
			at.Y, err = top.Add(-glyph.Descent)
			if err != nil {
				return layout.Glyph{}, err
			}
			maxAdvance = max(maxAdvance, glyph.Advance)
		default:
			return layout.Glyph{}, fmt.Errorf("%w: unknown assembly axis", layout.ErrInvalidInput)
		}
		placed[i] = layout.PositionedGlyph{Glyph: glyph, At: at}
		cursor, err = cursor.Add(advances[i])
		if err != nil {
			return layout.Glyph{}, err
		}
	}
	result := base
	result.FontID = info.ID
	result.ID = uint16(mapped.ID)
	result.Text = text
	result.Outline = nil
	result.Parts = placed
	result.ExactGlyphRef = false
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
	switch axis {
	case font.HorizontalAxis:
		result.Advance = cursor
		result.Ascent = maxAscent
		result.Descent = maxDescent
		result.TopAccent = result.Advance / 2
		result.HasTopAccent = true
	case font.VerticalAxis:
		half, err := fullExtent.MulDiv(1, 2)
		if err != nil {
			return layout.Glyph{}, err
		}
		result.Ascent, err = half.Add(a.metrics.AxisHeight)
		if err != nil {
			return layout.Glyph{}, err
		}
		result.Descent, err = half.Add(-a.metrics.AxisHeight)
		if err != nil {
			return layout.Glyph{}, err
		}
		if result.Descent < 0 {
			result.Ascent = fullExtent
			result.Descent = 0
		}
		result.Advance = maxAdvance
	}
	return result, nil
}

func subEm(a, b geometry.Em) (geometry.Em, error) {
	if b == math.MinInt64 {
		return 0, geometry.ErrOverflow
	}
	return a.Add(-b)
}

func (a *adapter) measureAssembly(
	ctx context.Context,
	parts []font.AssemblyPart,
	repeat int,
	minOverlap geometry.Em,
	upem uint16,
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
		for range count {
			if measure.count%64 == 0 {
				if err := ctx.Err(); err != nil {
					return assemblyMeasure{}, err
				}
			}
			if measure.count >= a.limits.MaxOutputItems {
				return assemblyMeasure{}, fmt.Errorf("%w: assembly part count", layout.ErrLimit)
			}
			if previous != nil {
				overlapUnits, err := connectorOverlap(*previous, *part)
				if err != nil {
					return assemblyMeasure{}, err
				}
				overlap, err := a.emFromUnits(overlapUnits, upem)
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
				advance, err := a.emFromUnits(int64(previous.FullAdvance), upem)
				if err != nil {
					return assemblyMeasure{}, err
				}
				advance, err = advance.Add(-overlap)
				if err != nil {
					return assemblyMeasure{}, err
				}
				measure.full, err = measure.full.Add(advance)
				if err != nil {
					return assemblyMeasure{}, err
				}
				growth, err := overlap.Add(-minOverlap)
				if err != nil {
					return assemblyMeasure{}, err
				}
				measure.growable, err = measure.growable.Add(growth)
				if err != nil {
					return assemblyMeasure{}, err
				}
			}
			previous = part
			measure.count++
		}
	}
	if previous != nil {
		advance, err := a.emFromUnits(int64(previous.FullAdvance), upem)
		if err != nil {
			return assemblyMeasure{}, err
		}
		measure.full, err = measure.full.Add(advance)
		if err != nil {
			return assemblyMeasure{}, err
		}
	}
	return measure, nil
}

func assemblyReaches(measure assemblyMeasure, target geometry.Em) bool {
	full := measure.full
	if full < target {
		growth, err := subEm(target, full)
		if err != nil || growth > measure.growable {
			growth = measure.growable
		}
		full, err = full.Add(growth)
		if err != nil {
			return false
		}
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

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
