package untie

import (
	"context"
	"math"
	"testing"

	"github.com/caelyreth/untie/display"
)

func TestRendererPixelGridSnapsThinRulesOnlyWhenEnabled(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	render := func(grid display.PixelGrid) *display.Box {
		t.Helper()
		result, err := renderer.Render(context.Background(), `\frac{1}{2}`, RenderRequest{
			Dialect: TeX,
			Style: display.Style{
				Size:      24,
				PixelGrid: grid,
			},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		return result.Box
	}
	plain := render(display.PixelGrid{})
	snapped := render(display.PixelGrid{Scale: 1, PixelRatio: 1})

	plainRules := ruleItems(plain)
	snappedRules := ruleItems(snapped)
	if len(plainRules) == 0 || len(snappedRules) != len(plainRules) {
		t.Fatalf("rule counts = plain %d, snapped %d", len(plainRules), len(snappedRules))
	}
	changed := false
	for i := range plainRules {
		before, after := plainRules[i], snappedRules[i]
		if before.Snap != display.SnapY {
			t.Fatalf("fraction rule snap hint = %d, want SnapY", before.Snap)
		}
		if after.Rect.Y != float32(math.Round(float64(after.Rect.Y))) ||
			after.Rect.Height != float32(math.Round(float64(after.Rect.Height))) {
			t.Fatalf("snapped rule is off the one-pixel grid: %#v", after.Rect)
		}
		changed = changed || before.Rect != after.Rect
	}
	if !changed {
		t.Fatal("enabled pixel grid did not change a thin fraction rule")
	}
	for _, item := range snapped.Items {
		if item.Kind != display.ItemRule && item.Snap != display.SnapNone {
			t.Fatalf("non-rule item has snap hint %d: %#v", item.Snap, item)
		}
	}
}

func TestRendererRejectsInvalidPixelGrid(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	_, err = renderer.Render(context.Background(), `x`, RenderRequest{
		Style: display.Style{
			Size:      16,
			PixelGrid: display.PixelGrid{Scale: 1},
		},
	})
	if err == nil {
		t.Fatal("Render() accepted a partial pixel grid")
	}
}

func ruleItems(box *display.Box) []display.Item {
	var rules []display.Item
	for _, item := range box.Items {
		if item.Kind == display.ItemRule {
			rules = append(rules, item)
		}
	}
	return rules
}
