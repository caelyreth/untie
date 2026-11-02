package fixture

import (
	"fmt"
	"math"
)

// DIPPerPoint is the CSS conversion factor for physical points.
const DIPPerPoint = 4.0 / 3.0

// TypletFrameTolerancePoints is Typlet's reviewed frame comparison tolerance.
const TypletFrameTolerancePoints = 0.0015

// TypletFrameToleranceDIP is the same tolerance after conversion to DIP.
const TypletFrameToleranceDIP = TypletFrameTolerancePoints * DIPPerPoint

// Points converts fixture points to device-independent pixels.
func PointsToDIP(points float64) (float64, error) {
	if math.IsNaN(points) || math.IsInf(points, 0) {
		return 0, fmt.Errorf("fixture: non-finite point value %v", points)
	}
	dip := points * DIPPerPoint
	if math.IsNaN(dip) || math.IsInf(dip, 0) {
		return 0, fmt.Errorf("fixture: point value %v overflows DIP", points)
	}
	return dip, nil
}

// DIPToPoints converts device-independent pixels to fixture points.
func DIPToPoints(dip float64) (float64, error) {
	if math.IsNaN(dip) || math.IsInf(dip, 0) {
		return 0, fmt.Errorf("fixture: non-finite DIP value %v", dip)
	}
	points := dip / DIPPerPoint
	if math.IsNaN(points) || math.IsInf(points, 0) {
		return 0, fmt.Errorf("fixture: DIP value %v overflows points", dip)
	}
	return points, nil
}

// FrameToDIP is the sole frame-unit adapter. Geometry, widths, positions,
// sizes, and stroke thicknesses become DIP. Linear transform coefficients and
// colors are unitless and remain unchanged.
func FrameToDIP(frame Frame) (Frame, error) {
	out := frame
	out.Fonts = append([]string(nil), frame.Fonts...)

	var err error
	if frame.Box != nil {
		box := *frame.Box
		box.Width, err = convertOptionalPoints(frame.Box.Width)
		if err != nil {
			return Frame{}, err
		}
		box.Height, err = convertOptionalPoints(frame.Box.Height)
		if err != nil {
			return Frame{}, err
		}
		box.Ascent, err = convertOptionalPoints(frame.Box.Ascent)
		if err != nil {
			return Frame{}, err
		}
		box.Descent, err = convertOptionalPoints(frame.Box.Descent)
		if err != nil {
			return Frame{}, err
		}
		out.Box = &box
	}
	out.FontSize, err = PointsToDIP(frame.FontSize)
	if err != nil {
		return Frame{}, err
	}
	out.Page[0], err = PointsToDIP(frame.Page[0])
	if err != nil {
		return Frame{}, fmt.Errorf("page width: %w", err)
	}
	out.Page[1], err = PointsToDIP(frame.Page[1])
	if err != nil {
		return Frame{}, fmt.Errorf("page height: %w", err)
	}

	out.Glyphs = make([]FrameGlyph, len(frame.Glyphs))
	for i, glyph := range frame.Glyphs {
		out.Glyphs[i] = glyph
		out.Glyphs[i].X, err = PointsToDIP(glyph.X)
		if err != nil {
			return Frame{}, fmt.Errorf("glyph %d x: %w", i, err)
		}
		out.Glyphs[i].Y, err = PointsToDIP(glyph.Y)
		if err != nil {
			return Frame{}, fmt.Errorf("glyph %d y: %w", i, err)
		}
		out.Glyphs[i].Size, err = PointsToDIP(glyph.Size)
		if err != nil {
			return Frame{}, fmt.Errorf("glyph %d size: %w", i, err)
		}
	}

	out.Shapes = make([]FrameShape, len(frame.Shapes))
	for i, shape := range frame.Shapes {
		out.Shapes[i] = shape
		if shape.Line != nil {
			out.Shapes[i].Line, err = convertPoints(shape.Line)
			if err != nil {
				return Frame{}, fmt.Errorf("shape %d line: %w", i, err)
			}
		}
		if shape.Rect != nil {
			out.Shapes[i].Rect, err = convertPoints(shape.Rect)
			if err != nil {
				return Frame{}, fmt.Errorf("shape %d rect: %w", i, err)
			}
		}
		if shape.Box != nil {
			out.Shapes[i].Box, err = convertPoints(shape.Box)
			if err != nil {
				return Frame{}, fmt.Errorf("shape %d box: %w", i, err)
			}
		}
		if shape.Stroke != nil {
			stroke := *shape.Stroke
			stroke.Thickness, err = PointsToDIP(shape.Stroke.Thickness)
			if err != nil {
				return Frame{}, fmt.Errorf("shape %d stroke: %w", i, err)
			}
			out.Shapes[i].Stroke = &stroke
		}
	}
	return out, nil
}

func convertOptionalPoints(value *float64) (*float64, error) {
	if value == nil {
		return nil, nil
	}
	converted, err := PointsToDIP(*value)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

func convertPoints(values []float64) ([]float64, error) {
	out := make([]float64, len(values))
	for i, value := range values {
		converted, err := PointsToDIP(value)
		if err != nil {
			return nil, fmt.Errorf("value %d: %w", i, err)
		}
		out[i] = converted
	}
	return out, nil
}
