package fixture

import (
	"errors"
	"fmt"
	"math"

	"github.com/caelyreth/untie/geometry"
)

// PathInfo describes a validated test-only vector path.
type PathInfo struct {
	Segments  int
	Subpaths  int
	Closed    int
	HasBounds bool
	Bounds    geometry.Rect
}

// InspectPath validates path operations and coordinates, and reports the
// control-point bounds. Curves are not evaluated to their tight ink bounds.
func InspectPath(path geometry.Path) (PathInfo, error) {
	info := PathInfo{}
	moved := false
	for i, segment := range path {
		info.Segments++
		switch segment.Op {
		case geometry.SegMoveTo:
			if !finitePoint(segment.Points[0]) {
				return PathInfo{}, invalidPathPoint(i, 0)
			}
			info.Subpaths++
			moved = true
			includePoint(&info, segment.Points[0])
		case geometry.SegLineTo:
			if !moved {
				return PathInfo{}, fmt.Errorf("fixture: path segment %d draws before move-to", i)
			}
			if !finitePoint(segment.Points[0]) {
				return PathInfo{}, invalidPathPoint(i, 0)
			}
			includePoint(&info, segment.Points[0])
		case geometry.SegQuadTo:
			if !moved {
				return PathInfo{}, fmt.Errorf("fixture: path segment %d draws before move-to", i)
			}
			for j := 0; j < 2; j++ {
				if !finitePoint(segment.Points[j]) {
					return PathInfo{}, invalidPathPoint(i, j)
				}
				includePoint(&info, segment.Points[j])
			}
		case geometry.SegCubeTo:
			if !moved {
				return PathInfo{}, fmt.Errorf("fixture: path segment %d draws before move-to", i)
			}
			for j := 0; j < 3; j++ {
				if !finitePoint(segment.Points[j]) {
					return PathInfo{}, invalidPathPoint(i, j)
				}
				includePoint(&info, segment.Points[j])
			}
		case geometry.SegClose:
			if !moved {
				return PathInfo{}, fmt.Errorf("fixture: path segment %d closes before move-to", i)
			}
			info.Closed++
		default:
			return PathInfo{}, fmt.Errorf("fixture: path segment %d has invalid operation %d", i, segment.Op)
		}
	}
	if len(path) == 0 {
		return PathInfo{}, errors.New("fixture: empty path")
	}
	return info, nil
}

func includePoint(info *PathInfo, point geometry.Point) {
	x, y := float64(point.X), float64(point.Y)
	if !info.HasBounds {
		info.Bounds = geometry.Rect{X: point.X, Y: point.Y}
		info.HasBounds = true
		return
	}
	minX := math.Min(float64(info.Bounds.X), x)
	minY := math.Min(float64(info.Bounds.Y), y)
	maxX := math.Max(float64(info.Bounds.X)+float64(info.Bounds.Width), x)
	maxY := math.Max(float64(info.Bounds.Y)+float64(info.Bounds.Height), y)
	info.Bounds.X = float32(minX)
	info.Bounds.Y = float32(minY)
	info.Bounds.Width = float32(maxX - minX)
	info.Bounds.Height = float32(maxY - minY)
}

func finitePoint(point geometry.Point) bool {
	return finite(float64(point.X)) && finite(float64(point.Y))
}

func invalidPathPoint(segment, point int) error {
	return fmt.Errorf("fixture: path segment %d point %d is non-finite", segment, point)
}
