package geometry

// Point is a two-dimensional point in final host units.
type Point struct {
	X float32
	Y float32
}

// Rect is an axis-aligned rectangle in final host units.
type Rect struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// SegOp is a path segment operation.
type SegOp uint8

const (
	SegMoveTo SegOp = iota
	SegLineTo
	SegQuadTo
	SegCubeTo
	SegClose
)

// Segment is one path command. The number of meaningful control points depends
// on Op: MoveTo/LineTo use one, QuadTo two, CubeTo three, and Close none.
type Segment struct {
	Op     SegOp
	Points [3]Point
}

// Path is a sequence of fillable path segments in host units.
type Path []Segment
