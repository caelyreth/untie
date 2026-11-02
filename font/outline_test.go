package font

import (
	"math"
	"testing"

	"github.com/caelyreth/untie/geometry"
	tsot "github.com/go-text/typesetting/font/opentype"
)

func TestDesignCoordinateQuantizesOnce(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value float32
		want  int32
	}{
		{0.49, 0},
		{0.5, 0},
		{1.49, 1},
		{1.5, 1},
		{-0.49, 0},
		{-0.5, 0},
		{-1.5, -1},
		{maxDesignCoordinate, maxDesignCoordinate},
		{-maxDesignCoordinate, -maxDesignCoordinate},
	}
	for _, tc := range tests {
		got, err := designCoordinate(tc.value)
		if err != nil {
			t.Errorf("designCoordinate(%v): %v", tc.value, err)
			continue
		}
		if got != tc.want {
			t.Errorf("designCoordinate(%v) = %d, want %d", tc.value, got, tc.want)
		}
	}
}

func TestDesignCoordinateRejectsNonFiniteAndOversized(t *testing.T) {
	t.Parallel()
	for _, value := range []float32{
		float32(math.NaN()),
		float32(math.Inf(1)),
		float32(math.Inf(-1)),
		maxDesignCoordinate * 2,
		-maxDesignCoordinate * 2,
	} {
		if got, err := designCoordinate(value); err == nil {
			t.Errorf("designCoordinate(%v) = %d, nil; want error", value, got)
		}
	}
}

func TestFractionalOutlineGeometryQuantizes(t *testing.T) {
	t.Parallel()
	segments := []tsot.Segment{
		{Op: tsot.SegmentOpMoveTo, Args: [3]tsot.SegmentPoint{{X: 10.49, Y: -3.5}}},
		{Op: tsot.SegmentOpQuadTo, Args: [3]tsot.SegmentPoint{{X: 14.5, Y: -7.49}, {X: 20.5, Y: -11.5}}},
		{Op: tsot.SegmentOpCubeTo, Args: [3]tsot.SegmentPoint{{X: -2.5, Y: 1.5}, {X: 25.49, Y: -20.49}, {X: 30.4, Y: -22.5}}},
	}
	bounds, err := outlineBounds(segments)
	if err != nil {
		t.Fatal(err)
	}
	want := Bounds{XMin: -2, YMin: -1, XMax: 30, YMax: 22}
	if bounds != want {
		t.Fatalf("outlineBounds = %#v, want %#v", bounds, want)
	}
	converted, err := convertSegments(segments)
	if err != nil {
		t.Fatal(err)
	}
	if len(converted) != 4 || converted[0].Points[0] != (Point{X: 10, Y: 3}) {
		t.Fatalf("convertSegments = %#v", converted)
	}
	if converted[1].Points[0] != (Point{X: 14, Y: 7}) ||
		converted[1].Points[1] != (Point{X: 20, Y: 11}) {
		t.Fatalf("quadratic points = %#v", converted[1].Points)
	}
	if converted[3].Op != geometry.SegClose {
		t.Fatalf("last operation = %d, want close", converted[3].Op)
	}
}

func TestConvertSegmentsRejectsInvalidOperation(t *testing.T) {
	t.Parallel()
	_, err := convertSegments([]tsot.Segment{{Op: tsot.SegmentOp(99)}})
	if err == nil {
		t.Fatal("convertSegments accepted an invalid operation")
	}
}
