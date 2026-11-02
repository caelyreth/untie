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
	var minX, minY, maxX, maxY float64
	for i, segment := range path {
		info.Segments++
		switch segment.Op {
		case geometry.SegMoveTo:
			if !finitePoint(segment.Points[0]) {
				return PathInfo{}, invalidPathPoint(i, 0)
			}
			info.Subpaths++
			moved = true
			includePoint(&info, segment.Points[0], &minX, &minY, &maxX, &maxY)
		case geometry.SegLineTo:
			if !moved {
				return PathInfo{}, fmt.Errorf("fixture: path segment %d draws before move-to", i)
			}
			if !finitePoint(segment.Points[0]) {
				return PathInfo{}, invalidPathPoint(i, 0)
			}
			includePoint(&info, segment.Points[0], &minX, &minY, &maxX, &maxY)
		case geometry.SegQuadTo:
			if !moved {
				return PathInfo{}, fmt.Errorf("fixture: path segment %d draws before move-to", i)
			}
			for j := 0; j < 2; j++ {
				if !finitePoint(segment.Points[j]) {
					return PathInfo{}, invalidPathPoint(i, j)
				}
				includePoint(&info, segment.Points[j], &minX, &minY, &maxX, &maxY)
			}
		case geometry.SegCubeTo:
			if !moved {
				return PathInfo{}, fmt.Errorf("fixture: path segment %d draws before move-to", i)
			}
			for j := 0; j < 3; j++ {
				if !finitePoint(segment.Points[j]) {
					return PathInfo{}, invalidPathPoint(i, j)
				}
				includePoint(&info, segment.Points[j], &minX, &minY, &maxX, &maxY)
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
	width, height := maxX-minX, maxY-minY
	if !finite(width) || width > math.MaxFloat32 || !finite(height) || height > math.MaxFloat32 {
		return PathInfo{}, fmt.Errorf("fixture: path bounds %.17g x %.17g exceed float32 range", width, height)
	}
	info.Bounds = geometry.Rect{
		X: float32(minX), Y: float32(minY),
		Width: float32(width), Height: float32(height),
	}
	return info, nil
}

func includePoint(info *PathInfo, point geometry.Point, minX, minY, maxX, maxY *float64) {
	x, y := float64(point.X), float64(point.Y)
	if !info.HasBounds {
		*minX, *minY, *maxX, *maxY = x, y, x, y
		info.HasBounds = true
		return
	}
	*minX = math.Min(*minX, x)
	*minY = math.Min(*minY, y)
	*maxX = math.Max(*maxX, x)
	*maxY = math.Max(*maxY, y)
}

func finitePoint(point geometry.Point) bool {
	return finite(float64(point.X)) && finite(float64(point.Y))
}

func invalidPathPoint(segment, point int) error {
	return fmt.Errorf("fixture: path segment %d point %d is non-finite", segment, point)
}
