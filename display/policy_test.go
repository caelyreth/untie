package display

import (
	"math"
	"testing"
)

func TestColorPolicyPreserveIsNoOpDefault(t *testing.T) {
	t.Parallel()

	paint := Paint{
		Role:        RoleInk,
		Explicit:    RGBA{R: 12, G: 34, B: 56, A: 78},
		HasExplicit: true,
	}
	var policy ColorPolicy
	if err := policy.Validate(); err != nil {
		t.Fatalf("zero policy validation: %v", err)
	}
	if got := policy.Resolve(paint); got != paint {
		t.Fatalf("preserve resolved paint = %#v, want %#v", got, paint)
	}
	role := Paint{Role: RoleError}
	if got := policy.Resolve(role); got != role {
		t.Fatalf("preserve changed semantic role: %#v", got)
	}
}

func TestColorPolicyMapsExplicitColorsToNearestRoles(t *testing.T) {
	t.Parallel()

	policy := ColorPolicy{
		Mode: ColorMapToRoles,
		Palette: ColorRolePalette{
			Ink:         RGBA{R: 20, G: 20, B: 20, A: 0xff},
			Error:       RGBA{R: 220, G: 20, B: 20, A: 0xff},
			Placeholder: RGBA{R: 130, G: 130, B: 130, A: 0xff},
		},
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("policy validation: %v", err)
	}
	tests := []struct {
		name  string
		color RGBA
		want  Role
	}{
		{name: "ink", color: RGBA{R: 24, G: 26, B: 30, A: 0xff}, want: RoleInk},
		{name: "error", color: RGBA{R: 200, G: 30, B: 25, A: 0xff}, want: RoleError},
		{name: "placeholder", color: RGBA{R: 120, G: 125, B: 128, A: 0xff}, want: RolePlaceholder},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			got := policy.Resolve(Paint{
				Role:        RoleInk,
				Explicit:    test.color,
				HasExplicit: true,
			})
			if got.HasExplicit || got.Role != test.want {
				t.Fatalf("resolved paint = %#v, want role %d", got, test.want)
			}
		})
	}
	role := Paint{Role: RolePlaceholder}
	if got := policy.Resolve(role); got != role {
		t.Fatalf("mapped semantic role: got %#v, want %#v", got, role)
	}
}

func TestColorPolicyAdjustsContrastAgainstLightAndDarkBackgrounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		color      RGBA
		background RGBA
	}{
		{
			name:       "dark text on dark background",
			color:      RGBA{R: 20, G: 20, B: 20, A: 0xff},
			background: RGBA{R: 10, G: 12, B: 16, A: 0xff},
		},
		{
			name:       "light text on light background",
			color:      RGBA{R: 240, G: 240, B: 240, A: 0xff},
			background: RGBA{R: 250, G: 248, B: 244, A: 0xff},
		},
		{
			name:       "semi-transparent text",
			color:      RGBA{R: 60, G: 60, B: 60, A: 200},
			background: RGBA{R: 8, G: 8, B: 12, A: 0xff},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			policy := ColorPolicy{
				Mode:        ColorAdjustContrast,
				Background:  test.background,
				MinContrast: 4.5,
			}
			if err := policy.Validate(); err != nil {
				t.Fatalf("policy validation: %v", err)
			}
			resolved := policy.Resolve(Paint{
				Role:        RoleInk,
				Explicit:    test.color,
				HasExplicit: true,
			})
			if !resolved.HasExplicit || resolved.Role != RoleInk {
				t.Fatalf("contrast result = %#v, want explicit ink paint", resolved)
			}
			if resolved.Explicit.A != test.color.A {
				t.Fatalf("alpha = %d, want preserved %d", resolved.Explicit.A, test.color.A)
			}
			if got := contrastAgainst(resolved.Explicit, test.background); got < 4.5 {
				t.Fatalf("contrast = %v, want at least 4.5; resolved %#v", got, resolved.Explicit)
			}
			if test.name == "dark text on dark background" &&
				resolved.Explicit == test.color {
				t.Fatal("low-contrast color was not adjusted")
			}
		})
	}
}

func TestColorPolicyValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  ColorPolicy
		wantErr bool
	}{
		{name: "preserve", policy: ColorPolicy{}},
		{name: "unknown mode", policy: ColorPolicy{Mode: 3}, wantErr: true},
		{name: "incomplete palette", policy: ColorPolicy{Mode: ColorMapToRoles}, wantErr: true},
		{
			name: "complete palette",
			policy: ColorPolicy{Mode: ColorMapToRoles, Palette: ColorRolePalette{
				Ink:         RGBA{A: 0xff},
				Error:       RGBA{R: 0xff, A: 0xff},
				Placeholder: RGBA{G: 0xff, A: 0xff},
			}},
		},
		{
			name:    "transparent background",
			policy:  ColorPolicy{Mode: ColorAdjustContrast},
			wantErr: true,
		},
		{
			name: "contrast too high",
			policy: ColorPolicy{
				Mode:        ColorAdjustContrast,
				Background:  RGBA{A: 0xff},
				MinContrast: 21.1,
			},
			wantErr: true,
		},
		{
			name: "contrast NaN",
			policy: ColorPolicy{
				Mode:        ColorAdjustContrast,
				Background:  RGBA{A: 0xff},
				MinContrast: float32(math.NaN()),
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			err := test.policy.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
