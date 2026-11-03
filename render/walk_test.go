package render_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/render"
)

func TestWalkStreamsVisibleItemsInOrder(t *testing.T) {
	t.Parallel()

	box := &display.Box{
		Items: []display.Item{
			{
				Kind:      display.ItemGlyphPath,
				Paint:     display.Paint{Role: display.RoleError},
				Bound:     geometry.Rect{X: 1, Y: 2, Width: 3, Height: 4},
				PathIndex: 0,
				At:        geometry.Point{X: 5, Y: 6},
				Em:        7,
			},
			{
				Kind:      display.ItemGlyphRef,
				Paint:     display.Paint{Role: display.RoleInk},
				PathIndex: display.NoPath,
				FontID:    "math",
				Glyph:     9,
			},
			{
				Kind:      display.ItemRule,
				Paint:     display.Paint{Role: display.RolePlaceholder},
				Bound:     geometry.Rect{X: 8, Y: 9, Width: 10, Height: 11},
				PathIndex: display.NoPath,
				Rect:      geometry.Rect{X: 8, Y: 9, Width: 10, Height: 11},
			},
			{
				Kind:      display.ItemGlyphPath,
				PathIndex: 1,
				At:        geometry.Point{X: 12, Y: 13},
				Em:        14,
			},
		},
		Paths: []display.PathTemplate{
			{
				Segments: []geometry.Segment{{
					Op:     geometry.SegMoveTo,
					Points: [3]geometry.Point{{X: 0.25, Y: 0.5}},
				}},
				Bound: geometry.Rect{X: 0.25, Y: 0.5, Width: 1, Height: 2},
			},
			{
				Segments: []geometry.Segment{{
					Op:     geometry.SegLineTo,
					Points: [3]geometry.Point{{X: 0.75, Y: 1.5}},
				}},
				Bound: geometry.Rect{X: 0.75, Y: 1.5, Width: 3, Height: 4},
			},
		},
	}
	before := box.Clone()

	var commands []render.Command
	err := render.Walk(box, func(command render.Command) error {
		commands = append(commands, command)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	if len(commands) != 3 {
		t.Fatalf("Walk() commands = %d, want 3", len(commands))
	}
	if got := []render.CommandKind{commands[0].Kind, commands[1].Kind, commands[2].Kind}; !reflect.DeepEqual(got, []render.CommandKind{
		render.CommandPath,
		render.CommandRule,
		render.CommandPath,
	}) {
		t.Fatalf("Walk() kinds = %v", got)
	}
	if commands[0].Paint.Role != display.RoleError ||
		commands[0].Bound != (geometry.Rect{X: 1, Y: 2, Width: 3, Height: 4}) ||
		commands[0].At != (geometry.Point{X: 5, Y: 6}) ||
		commands[0].Em != 7 {
		t.Fatalf("first path command = %+v", commands[0])
	}
	if commands[1].Paint.Role != display.RolePlaceholder ||
		commands[1].Rect != (geometry.Rect{X: 8, Y: 9, Width: 10, Height: 11}) {
		t.Fatalf("rule command = %+v", commands[1])
	}
	if commands[2].At != (geometry.Point{X: 12, Y: 13}) || commands[2].Em != 14 {
		t.Fatalf("second path command = %+v", commands[2])
	}
	if &commands[0].Path.Segments[0] != &box.Paths[0].Segments[0] {
		t.Fatal("Walk() copied the first path template")
	}
	if &commands[2].Path.Segments[0] != &box.Paths[1].Segments[0] {
		t.Fatal("Walk() copied the second path template")
	}
	if commands[0].Path.Segments[0].Points[0] != (geometry.Point{X: 0.25, Y: 0.5}) {
		t.Fatal("Walk() transformed the borrowed local-em point")
	}
	if !reflect.DeepEqual(box, before) {
		t.Fatal("Walk() mutated the box")
	}
}

func TestWalkPropagatesCallbackErrorAndStops(t *testing.T) {
	t.Parallel()

	stop := errors.New("stop")
	box := &display.Box{Items: []display.Item{
		{Kind: display.ItemRule, PathIndex: display.NoPath},
		{Kind: display.ItemRule, PathIndex: display.NoPath},
	}}
	visited := 0
	err := render.Walk(box, func(render.Command) error {
		visited++
		return stop
	})
	if err != stop {
		t.Fatalf("Walk() error = %v, want unchanged callback error %v", err, stop)
	}
	if !errors.Is(err, stop) {
		t.Fatalf("Walk() error = %v, want %v", err, stop)
	}
	if visited != 1 {
		t.Fatalf("Walk() visited %d commands, want 1", visited)
	}
}

func TestWalkRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	if err := render.Walk(&display.Box{}, nil); !errors.Is(err, render.ErrInvalidCallback) {
		t.Fatalf("Walk(nil callback) error = %v", err)
	}
	if err := render.Walk(nil, func(render.Command) error { return nil }); !errors.Is(err, render.ErrInvalidBox) {
		t.Fatalf("Walk(nil box) error = %v", err)
	}
	invalidPath := &display.Box{Items: []display.Item{{
		Kind:      display.ItemGlyphPath,
		PathIndex: 1,
	}}}
	if err := render.Walk(invalidPath, func(render.Command) error { return nil }); !errors.Is(err, render.ErrInvalidBox) {
		t.Fatalf("Walk(invalid path index) error = %v", err)
	}
	negativePath := &display.Box{Items: []display.Item{{
		Kind:      display.ItemGlyphPath,
		PathIndex: -1,
	}}}
	if err := render.Walk(negativePath, func(render.Command) error { return nil }); !errors.Is(err, render.ErrInvalidBox) {
		t.Fatalf("Walk(negative path index) error = %v", err)
	}
	invalidKind := &display.Box{Items: []display.Item{{Kind: display.ItemKind(255)}}}
	if err := render.Walk(invalidKind, func(render.Command) error { return nil }); !errors.Is(err, render.ErrInvalidBox) {
		t.Fatalf("Walk(invalid item kind) error = %v", err)
	}
}

func TestResolvePathMaterializesEverySegmentOperation(t *testing.T) {
	t.Parallel()

	command := render.Command{
		Kind: render.CommandPath,
		Path: display.PathTemplate{
			Segments: []geometry.Segment{
				{Op: geometry.SegMoveTo, Points: [3]geometry.Point{{X: 1, Y: 2}}},
				{Op: geometry.SegLineTo, Points: [3]geometry.Point{{X: 3, Y: 4}}},
				{Op: geometry.SegQuadTo, Points: [3]geometry.Point{{X: 5, Y: 6}, {X: 7, Y: 8}}},
				{Op: geometry.SegCubeTo, Points: [3]geometry.Point{{X: 9, Y: 10}, {X: 11, Y: 12}, {X: 13, Y: 14}}},
				{Op: geometry.SegClose},
			},
		},
		At: geometry.Point{X: 10, Y: -2},
		Em: 2,
	}
	before := append([]geometry.Segment(nil), command.Path.Segments...)
	path, err := render.ResolvePath(command)
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}
	want := geometry.Path{
		{Op: geometry.SegMoveTo, Points: [3]geometry.Point{{X: 12, Y: 2}}},
		{Op: geometry.SegLineTo, Points: [3]geometry.Point{{X: 16, Y: 6}}},
		{Op: geometry.SegQuadTo, Points: [3]geometry.Point{{X: 20, Y: 10}, {X: 24, Y: 14}}},
		{Op: geometry.SegCubeTo, Points: [3]geometry.Point{{X: 28, Y: 18}, {X: 32, Y: 22}, {X: 36, Y: 26}}},
		{Op: geometry.SegClose},
	}
	if !reflect.DeepEqual(path, want) {
		t.Fatalf("ResolvePath() = %#v, want %#v", path, want)
	}
	if !reflect.DeepEqual(command.Path.Segments, before) {
		t.Fatal("ResolvePath() mutated the borrowed template")
	}
	if &path[0] == &command.Path.Segments[0] {
		t.Fatal("ResolvePath() returned borrowed storage")
	}
	resolvedAgain, err := render.ResolvePath(command)
	if err != nil {
		t.Fatalf("second ResolvePath() error = %v", err)
	}
	if &resolvedAgain[0] == &path[0] {
		t.Fatal("ResolvePath() reused prior result storage")
	}
}

func TestResolvePathRejectsInvalidCommand(t *testing.T) {
	t.Parallel()

	valid := render.Command{
		Kind: render.CommandPath,
		Path: display.PathTemplate{Segments: []geometry.Segment{{
			Op:     geometry.SegMoveTo,
			Points: [3]geometry.Point{{X: 1, Y: 1}},
		}}},
		Em: 1,
	}
	tests := []struct {
		name    string
		command render.Command
	}{
		{name: "rule", command: render.Command{Kind: render.CommandRule}},
		{name: "empty", command: render.Command{Kind: render.CommandPath, Em: 1}},
		{name: "zero em", command: render.Command{Kind: render.CommandPath, Path: valid.Path}},
		{name: "negative em", command: render.Command{Kind: render.CommandPath, Path: valid.Path, Em: -1}},
		{name: "nan em", command: render.Command{Kind: render.CommandPath, Path: valid.Path, Em: float32(math.NaN())}},
		{name: "infinite at", command: render.Command{
			Kind: render.CommandPath,
			Path: valid.Path,
			At:   geometry.Point{X: float32(math.Inf(1))},
			Em:   1,
		}},
		{name: "unknown op", command: render.Command{
			Kind: render.CommandPath,
			Path: display.PathTemplate{Segments: []geometry.Segment{{Op: geometry.SegOp(255)}}},
			Em:   1,
		}},
		{name: "non-finite point", command: render.Command{
			Kind: render.CommandPath,
			Path: display.PathTemplate{Segments: []geometry.Segment{{
				Op:     geometry.SegLineTo,
				Points: [3]geometry.Point{{X: float32(math.NaN())}},
			}}},
			Em: 1,
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path, err := render.ResolvePath(test.command)
			if !errors.Is(err, render.ErrInvalidCommand) {
				t.Fatalf("ResolvePath() error = %v", err)
			}
			if path != nil {
				t.Fatalf("ResolvePath() path = %#v, want nil", path)
			}
		})
	}
}
