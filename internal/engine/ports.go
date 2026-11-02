package engine

import (
	"math"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (a *adapter) designPath(segments []font.Segment, upem uint16) (layout.Path, error) {
	path := make(layout.Path, len(segments))
	for i, segment := range segments {
		path[i].Op = segment.Op
		for j, point := range segment.Points {
			x, err := a.emFromUnits(int64(point.X), upem)
			if err != nil {
				return nil, err
			}
			y, err := a.emFromUnits(int64(point.Y), upem)
			if err != nil {
				return nil, err
			}
			path[i].Points[j] = layout.Point{X: x, Y: y}
		}
	}
	return path, nil
}

func (a *adapter) centerOnAxis(glyph layout.Glyph) (layout.Glyph, error) {
	height, err := glyph.Ascent.Add(glyph.Descent)
	if err != nil {
		return layout.Glyph{}, err
	}
	half, err := height.MulDiv(1, 2)
	if err != nil {
		return layout.Glyph{}, err
	}
	desiredDescent, err := half.Add(-a.metrics.AxisHeight)
	if err != nil {
		return layout.Glyph{}, err
	}
	shift, err := desiredDescent.Add(-glyph.Descent)
	if err != nil {
		return layout.Glyph{}, err
	}
	if shift != 0 {
		for i := range glyph.Outline {
			for j := range glyph.Outline[i].Points {
				glyph.Outline[i].Points[j].Y, err = glyph.Outline[i].Points[j].Y.Add(shift)
				if err != nil {
					return layout.Glyph{}, err
				}
			}
		}
	}
	if glyph.HasInkBottom {
		glyph.InkBottom, err = glyph.InkBottom.Add(shift)
		if err != nil {
			return layout.Glyph{}, err
		}
	}
	glyph.Ascent, err = height.Add(-desiredDescent)
	if err != nil {
		return layout.Glyph{}, err
	}
	glyph.Descent = desiredDescent
	return glyph, nil
}

func (a *adapter) buildFaceOrder(role ir.FontRole) []font.FaceRef {
	faces := make([]font.FaceRef, 0, 2+a.fonts.FaceCount())
	appendFace := func(ref font.FaceRef) {
		for _, existing := range faces {
			if existing == ref {
				return
			}
		}
		if _, ok := a.fonts.Face(ref); ok {
			faces = append(faces, ref)
		}
	}
	if role == ir.FontText {
		appendFace(font.TextFace)
	}
	appendFace(font.MathFace)
	for i := 0; ; i++ {
		ref, ok := font.FallbackFace(i)
		if !ok {
			break
		}
		if _, exists := a.fonts.Face(ref); !exists {
			break
		}
		appendFace(ref)
	}
	return faces
}

func (a *adapter) faceOrder(role ir.FontRole) []font.FaceRef {
	if role == ir.FontText {
		return a.textFaces
	}
	return a.mathFaces
}

func (a *adapter) mapTextAcrossFaces(faces []font.FaceRef, text string) (font.Glyph, font.FaceRef, bool) {
	for _, face := range faces {
		if mapped, ok := a.mapText(face, text); ok {
			return mapped, face, true
		}
	}
	return font.Glyph{}, 0, false
}

func (a *adapter) mapText(face font.FaceRef, text string) (font.Glyph, bool) {
	runes := []rune(text)
	if len(runes) == 1 {
		return a.fonts.MapRune(face, runes[0])
	}
	if len(runes) == 2 && isVariationSelector(runes[1]) {
		return a.fonts.MapVariation(face, runes[0], runes[1])
	}
	return font.Glyph{}, false
}

func (a *adapter) emFromUnits(value int64, upem uint16) (geometry.Em, error) {
	em, err := geometry.FontUnitsToEm(value, int64(upem))
	if err != nil {
		return 0, err
	}
	max := a.maxEm()
	if int64(em) > max || int64(em) < -max {
		return 0, geometry.ErrDimension
	}
	return em, nil
}

func (a *adapter) maxEm() int64 {
	max := a.limits.MaxDimension
	if max <= 0 {
		max = source.DefaultLimits().MaxDimension
	}
	if max > math.MaxInt64/geometry.EmScale {
		return math.MaxInt64
	}
	return max * geometry.EmScale
}

func maxEm(a, b geometry.Em) geometry.Em {
	if a > b {
		return a
	}
	return b
}

func (a *adapter) toDesignUnits(value geometry.Em, upem uint16) (int32, error) {
	scaled := float64(value) * float64(upem) / float64(geometry.EmScale)
	if math.IsNaN(scaled) || math.IsInf(scaled, 0) {
		return 0, geometry.ErrInvalidFloat
	}
	rounded := math.Round(scaled)
	if rounded < math.MinInt32 || rounded > math.MaxInt32 {
		return 0, geometry.ErrOverflow
	}
	return int32(rounded), nil
}
