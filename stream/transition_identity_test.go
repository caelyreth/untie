package stream

import (
	"testing"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
)

func TestRuleIdentityIgnoresCurrentGeometry(t *testing.T) {
	t.Parallel()

	previous, err := extractShapes(&untie.Box{Items: []display.Item{{
		Kind:  display.ItemRule,
		Paint: display.Paint{Role: display.RoleInk},
		Rect:  geometry.Rect{X: 10, Y: 20, Width: 30, Height: 1},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	current, err := extractShapes(&untie.Box{Items: []display.Item{{
		Kind:  display.ItemRule,
		Paint: display.Paint{Role: display.RoleInk},
		Rect:  geometry.Rect{X: 10, Y: 20, Width: 70, Height: 1},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if previous[0].key != current[0].key {
		t.Fatalf("resized rule identity changed: %q != %q",
			previous[0].key, current[0].key)
	}
}

func TestRuleFallbackSeparatesVisibleAndErrorPaint(t *testing.T) {
	t.Parallel()

	previous, err := extractShapes(&untie.Box{Items: []display.Item{{
		Kind:  display.ItemRule,
		Paint: display.Paint{Role: display.RoleError},
		Rect:  geometry.Rect{X: 10, Y: 20, Width: 30, Height: 1},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	current, err := extractShapes(&untie.Box{Items: []display.Item{{
		Kind:  display.ItemRule,
		Paint: display.Paint{Role: display.RoleInk},
		Rect:  geometry.Rect{X: 10, Y: 20, Width: 30, Height: 1},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if fallbackCompatible(previous[0], current[0]) {
		t.Fatal("visible rule fallback-matched an error rule")
	}
}

func TestErrorRuleCountDoesNotChangeVisibleRuleIdentity(t *testing.T) {
	t.Parallel()

	previous, err := extractShapes(&untie.Box{Items: []display.Item{
		{
			Kind:  display.ItemRule,
			Paint: display.Paint{Role: display.RoleError},
			Rect:  geometry.Rect{X: 10, Y: 20, Width: 30, Height: 1},
		},
		{
			Kind:  display.ItemRule,
			Paint: display.Paint{Role: display.RoleInk},
			Rect:  geometry.Rect{X: 100, Y: 20, Width: 30, Height: 1},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	current, err := extractShapes(&untie.Box{Items: []display.Item{
		{
			Kind:  display.ItemRule,
			Paint: display.Paint{Role: display.RoleError},
			Rect:  geometry.Rect{X: 10, Y: 20, Width: 30, Height: 1},
		},
		{
			Kind:  display.ItemRule,
			Paint: display.Paint{Role: display.RoleError},
			Rect:  geometry.Rect{X: 50, Y: 20, Width: 30, Height: 1},
		},
		{
			Kind:  display.ItemRule,
			Paint: display.Paint{Role: display.RoleInk},
			Rect:  geometry.Rect{X: 100, Y: 20, Width: 70, Height: 1},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if previous[1].key != current[2].key {
		t.Fatalf("visible rule identity shifted with error count: %q != %q",
			previous[1].key, current[2].key)
	}
}

func TestVisibleRuleKeepsMovingAsErrorRulesChange(t *testing.T) {
	t.Parallel()

	previous := &untie.Box{Items: []display.Item{
		{
			Kind:  display.ItemRule,
			Paint: display.Paint{Role: display.RoleError},
			Rect:  geometry.Rect{X: 10, Y: 20, Width: 30, Height: 1},
		},
		{
			Kind:  display.ItemRule,
			Paint: display.Paint{Role: display.RoleInk},
			Rect:  geometry.Rect{X: 100, Y: 20, Width: 30, Height: 1},
		},
	}}
	current := &untie.Box{Items: []display.Item{
		{
			Kind:  display.ItemRule,
			Paint: display.Paint{Role: display.RoleError},
			Rect:  geometry.Rect{X: 10, Y: 20, Width: 30, Height: 1},
		},
		{
			Kind:  display.ItemRule,
			Paint: display.Paint{Role: display.RoleError},
			Rect:  geometry.Rect{X: 50, Y: 20, Width: 30, Height: 1},
		},
		{
			Kind:  display.ItemRule,
			Paint: display.Paint{Role: display.RoleInk},
			Rect:  geometry.Rect{X: 100, Y: 20, Width: 70, Height: 1},
		},
	}}
	transition, err := NewTransition(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	var visible []Command
	if err := transition.Walk(0.5, 0.72, func(command Command) error {
		if command.Kind == Rule && command.Paint.Role == display.RoleInk {
			visible = append(visible, command)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(visible) != 1 {
		t.Fatalf("visible rules = %d, want one", len(visible))
	}
	if visible[0].Opacity != 1 {
		t.Fatalf("visible rule re-entered at opacity %v", visible[0].Opacity)
	}
	if visible[0].Rect.Width <= 30 || visible[0].Rect.Width >= 70 {
		t.Fatalf("visible rule width = %v, want interpolated", visible[0].Rect.Width)
	}
}
