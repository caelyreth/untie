package layout

import (
	"fmt"
	"math"

	"github.com/caelyreth/untie/geometry"
)

const circleKappa geometry.Em = 36_195 // 0.5522847498 em

type pathBuilder struct {
	path Path
}

func (b *pathBuilder) moveTo(x, y geometry.Em) {
	b.path = append(b.path, Segment{
		Op:     geometry.SegMoveTo,
		Points: [3]Point{{X: x, Y: y}},
	})
}

func (b *pathBuilder) lineTo(x, y geometry.Em) {
	b.path = append(b.path, Segment{
		Op:     geometry.SegLineTo,
		Points: [3]Point{{X: x, Y: y}},
	})
}

func (b *pathBuilder) cubeTo(x1, y1, x2, y2, x, y geometry.Em) {
	b.path = append(b.path, Segment{
		Op: geometry.SegCubeTo,
		Points: [3]Point{
			{X: x1, Y: y1},
			{X: x2, Y: y2},
			{X: x, Y: y},
		},
	})
}

func (b *pathBuilder) close() {
	b.path = append(b.path, Segment{Op: geometry.SegClose})
}

func rectPath(x, y, width, height geometry.Em, reverse bool) Path {
	var b pathBuilder
	b.moveTo(x, y)
	if reverse {
		b.lineTo(x, y+height)
		b.lineTo(x+width, y+height)
		b.lineTo(x+width, y)
	} else {
		b.lineTo(x+width, y)
		b.lineTo(x+width, y+height)
		b.lineTo(x, y+height)
	}
	b.close()
	return b.path
}

// roundedRectPath builds one closed contour. reverse emits the opposite
// winding so it can form a hole under either even-odd or nonzero filling.
func roundedRectPath(x, y, width, height, radius geometry.Em, reverse bool) (Path, error) {
	if width < 0 || height < 0 || radius < 0 {
		return nil, fmt.Errorf("%w: negative rounded rectangle dimension", ErrInvalidInput)
	}
	maxRadius := minEm(width, height) / 2
	radius = minEm(radius, maxRadius)
	if radius == 0 {
		return rectPath(x, y, width, height, reverse), nil
	}
	kx, err := mulDiv(radius, int64(circleKappa), int64(geometry.EmScale))
	if err != nil {
		return nil, err
	}

	var b pathBuilder
	b.moveTo(x+radius, y)
	if reverse {
		b.cubeTo(x+radius-kx, y, x, y+radius-kx, x, y+radius)
		b.lineTo(x, y+height-radius)
		b.cubeTo(x, y+height-radius+kx, x+radius-kx, y+height, x+radius, y+height)
		b.lineTo(x+width-radius, y+height)
		b.cubeTo(x+width-radius+kx, y+height, x+width, y+height-radius+kx, x+width, y+height-radius)
		b.lineTo(x+width, y+radius)
		b.cubeTo(x+width, y+radius-kx, x+width-radius+kx, y, x+width-radius, y)
		b.lineTo(x+radius, y)
	} else {
		b.lineTo(x+width-radius, y)
		b.cubeTo(x+width-radius+kx, y, x+width, y+radius-kx, x+width, y+radius)
		b.lineTo(x+width, y+height-radius)
		b.cubeTo(x+width, y+height-radius+kx, x+width-radius+kx, y+height, x+width-radius, y+height)
		b.lineTo(x+radius, y+height)
		b.cubeTo(x+radius-kx, y+height, x, y+height-radius+kx, x, y+height-radius)
		b.lineTo(x, y+radius)
		b.cubeTo(x, y+radius-kx, x+radius-kx, y, x+radius, y)
	}
	b.close()
	return b.path, nil
}

// lineQuad returns a butt-capped rectangular stroke centered on start/end.
func lineQuad(start, end Point, thickness geometry.Em) (Path, error) {
	if thickness < 0 {
		return nil, fmt.Errorf("%w: negative stroke thickness", ErrInvalidInput)
	}
	if thickness == 0 {
		return nil, nil
	}
	dx := end.X - start.X
	dy := end.Y - start.Y
	if dx == 0 && dy == 0 {
		return nil, nil
	}
	length := math.Hypot(float64(dx), float64(dy))
	if length == 0 || math.IsNaN(length) || math.IsInf(length, 0) {
		return nil, fmt.Errorf("%w: invalid stroke length", ErrInvalidInput)
	}
	half := thickness / 2
	nx := geometry.Em(math.Round(float64(-dy) / length * float64(half)))
	ny := geometry.Em(math.Round(float64(dx) / length * float64(half)))

	var b pathBuilder
	b.moveTo(start.X+nx, start.Y+ny)
	b.lineTo(end.X+nx, end.Y+ny)
	b.lineTo(end.X-nx, end.Y-ny)
	b.lineTo(start.X-nx, start.Y-ny)
	b.close()
	return b.path, nil
}

func pathInkBottom(path Path) (geometry.Em, bool) {
	var bottom geometry.Em
	havePoint := false
	for _, segment := range path {
		pointCount, ok := segmentPointCount(segment.Op)
		if !ok {
			continue
		}
		for i := 0; i < pointCount; i++ {
			y := segment.Points[i].Y
			if !havePoint || y > bottom {
				bottom = y
				havePoint = true
			}
		}
	}
	return bottom, havePoint
}

func minEm(a, b geometry.Em) geometry.Em {
	if a < b {
		return a
	}
	return b
}
