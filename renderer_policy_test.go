package untie

import (
	"context"
	"testing"

	"github.com/caelyreth/untie/display"
)

func TestRendererColorPolicyAppliesToSerializableDisplayList(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	request := RenderRequest{
		Dialect: TeX,
		Style: display.Style{
			Size: 24,
			ColorPolicy: display.ColorPolicy{
				Mode: display.ColorMapToRoles,
				Palette: display.ColorRolePalette{
					Ink:         display.RGBA{G: 0xff, A: 0xff},
					Error:       display.RGBA{R: 0xff, A: 0xff},
					Placeholder: display.RGBA{B: 0xff, A: 0xff},
				},
			},
		},
	}
	result, err := renderer.Render(
		context.Background(),
		`\textcolor{red}{x}`,
		request,
	)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(result.Box.Items) == 0 {
		t.Fatal("Render() returned no items")
	}
	for _, item := range result.Box.Items {
		if item.Paint.HasExplicit {
			t.Fatalf("item retained explicit paint after role mapping: %#v", item.Paint)
		}
	}
	if _, err := result.Box.MarshalCanonical(); err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}

	preserved, err := renderer.Render(context.Background(), `\textcolor{red}{x}`, RenderRequest{
		Dialect: TeX,
		Style:   display.Style{Size: 24},
	})
	if err != nil {
		t.Fatalf("default Render() error = %v", err)
	}
	foundExplicit := false
	for _, item := range preserved.Box.Items {
		foundExplicit = foundExplicit || item.Paint.HasExplicit
	}
	if !foundExplicit {
		t.Fatal("zero-value color policy did not preserve authored explicit paint")
	}
}

func TestRendererRejectsInvalidColorPolicy(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	_, err = renderer.Render(context.Background(), `x`, RenderRequest{
		Style: display.Style{
			Size: 16,
			ColorPolicy: display.ColorPolicy{
				Mode: display.ColorMapToRoles,
			},
		},
	})
	if err == nil {
		t.Fatal("Render() accepted an incomplete role palette")
	}
}
