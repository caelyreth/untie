package display

import (
	"fmt"
	"math"
)

// ColorPolicyMode selects how authored explicit colors participate in output.
type ColorPolicyMode uint8

const (
	// ColorPreserve leaves every authored explicit color unchanged. It is the
	// zero-value policy and the default behavior.
	ColorPreserve ColorPolicyMode = iota
	// ColorMapToRoles replaces explicit colors with the perceptually nearest
	// semantic role in ColorPolicy.Palette.
	ColorMapToRoles
	// ColorAdjustContrast minimally adjusts explicit colors until they meet
	// ColorPolicy.MinContrast against ColorPolicy.Background.
	ColorAdjustContrast
)

// ColorRolePalette supplies concrete colors for semantic paint roles when
// explicit colors are mapped back to roles.
type ColorRolePalette struct {
	Ink         RGBA
	Error       RGBA
	Placeholder RGBA
}

// ColorPolicy controls output-time treatment of authored explicit colors.
//
// Semantic role paints are never changed. Preserve is the zero value and keeps
// explicit colors byte-for-byte unchanged. MapToRoles discards authored color
// and alpha and emits a serializable semantic role. AdjustContrast preserves
// alpha and changes only the RGB needed to reach MinContrast against an opaque
// Background. If the preserved alpha makes the threshold impossible, it moves
// to the highest-contrast RGB endpoint. A zero MinContrast uses the WCAG AA
// normal-text threshold 4.5.
type ColorPolicy struct {
	Mode        ColorPolicyMode
	Palette     ColorRolePalette
	Background  RGBA
	MinContrast float32
}

// DefaultMinContrast is the WCAG AA contrast threshold used when
// ColorPolicy.MinContrast is zero.
const DefaultMinContrast float32 = 4.5

// Validate checks a color policy before layout.
func (p ColorPolicy) Validate() error {
	switch p.Mode {
	case ColorPreserve:
		return nil
	case ColorMapToRoles:
		if p.Palette.Ink.A == 0 || p.Palette.Error.A == 0 || p.Palette.Placeholder.A == 0 {
			return fmt.Errorf("map-to-roles requires an opaque-defined ink, error, and placeholder palette")
		}
		return nil
	case ColorAdjustContrast:
		if p.Background.A != 0xff {
			return fmt.Errorf("contrast policy requires an opaque background")
		}
		minimum := p.MinContrast
		if minimum == 0 {
			minimum = DefaultMinContrast
		}
		value := float64(minimum)
		if math.IsNaN(value) || math.IsInf(value, 0) || minimum < 1 || minimum > 21 {
			return fmt.Errorf("minimum contrast %v is outside [1,21]", p.MinContrast)
		}
		return nil
	default:
		return fmt.Errorf("unknown color policy mode %d", p.Mode)
	}
}

// Resolve returns paint with this policy applied.
//
// Callers must Validate the policy first. Preserve and unknown modes are
// intentionally no-ops so an unvalidated zero-value policy cannot corrupt
// output.
func (p ColorPolicy) Resolve(paint Paint) Paint {
	if !paint.HasExplicit {
		return paint
	}
	switch p.Mode {
	case ColorMapToRoles:
		return Paint{Role: p.nearestRole(paint.Explicit)}
	case ColorAdjustContrast:
		return Paint{
			Role:        RoleInk,
			Explicit:    p.adjustContrast(paint.Explicit),
			HasExplicit: true,
		}
	default:
		return paint
	}
}

func (p ColorPolicy) nearestRole(color RGBA) Role {
	l, a, b := oklab(color)
	roles := [...]struct {
		role  Role
		color RGBA
	}{
		{RoleInk, p.Palette.Ink},
		{RoleError, p.Palette.Error},
		{RolePlaceholder, p.Palette.Placeholder},
	}
	best := roles[0].role
	bestDistance := math.Inf(1)
	for _, candidate := range roles {
		cl, ca, cb := oklab(candidate.color)
		dl, da, db := l-cl, a-ca, b-cb
		distance := dl*dl + da*da + db*db
		if distance < bestDistance {
			best = candidate.role
			bestDistance = distance
		}
	}
	return best
}

func (p ColorPolicy) adjustContrast(color RGBA) RGBA {
	if color.A == 0 {
		return color
	}
	minimum := p.MinContrast
	if minimum == 0 {
		minimum = DefaultMinContrast
	}
	if contrastAgainst(color, p.Background) >= float64(minimum) {
		return color
	}

	black := color
	black.R, black.G, black.B = 0, 0, 0
	white := color
	white.R, white.G, white.B = 0xff, 0xff, 0xff
	target := black
	targetContrast := contrastAgainst(black, p.Background)
	if whiteContrast := contrastAgainst(white, p.Background); whiteContrast > targetContrast {
		target = white
		targetContrast = whiteContrast
	}
	if targetContrast < float64(minimum) {
		return target
	}

	// Contrast is monotonic while moving toward the higher-contrast endpoint.
	// Find the smallest blend that clears the requested threshold.
	low, high := 0.0, 1.0
	var adjusted RGBA
	for range 20 {
		middle := (low + high) / 2
		adjusted = blendRGB(color, target, middle)
		if contrastAgainst(adjusted, p.Background) >= float64(minimum) {
			high = middle
		} else {
			low = middle
		}
	}
	adjusted = blendRGB(color, target, high)
	if contrastAgainst(adjusted, p.Background) < float64(minimum) {
		return target
	}
	return adjusted
}

func blendRGB(from, to RGBA, amount float64) RGBA {
	blend := func(a, b uint8) uint8 {
		value := float64(a) + (float64(b)-float64(a))*amount
		return uint8(math.Floor(value + 0.5))
	}
	return RGBA{
		R: blend(from.R, to.R),
		G: blend(from.G, to.G),
		B: blend(from.B, to.B),
		A: from.A,
	}
}

func contrastAgainst(foreground, background RGBA) float64 {
	r, g, b := compositeRGB(foreground, background)
	foregroundLuminance := relativeLuminance(r, g, b)
	backgroundLuminance := relativeLuminance(
		float64(background.R)/255,
		float64(background.G)/255,
		float64(background.B)/255,
	)
	lighter := math.Max(foregroundLuminance, backgroundLuminance)
	darker := math.Min(foregroundLuminance, backgroundLuminance)
	return (lighter + 0.05) / (darker + 0.05)
}

func compositeRGB(foreground, background RGBA) (float64, float64, float64) {
	alpha := float64(foreground.A) / 255
	blend := func(front, back uint8) float64 {
		return (float64(front)*alpha + float64(back)*(1-alpha)) / 255
	}
	return blend(foreground.R, background.R),
		blend(foreground.G, background.G),
		blend(foreground.B, background.B)
}

func relativeLuminance(r, g, b float64) float64 {
	linear := func(value float64) float64 {
		if value <= 0.04045 {
			return value / 12.92
		}
		return math.Pow((value+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}

func oklab(color RGBA) (float64, float64, float64) {
	r := srgbToLinear(float64(color.R) / 255)
	g := srgbToLinear(float64(color.G) / 255)
	b := srgbToLinear(float64(color.B) / 255)

	l := 0.4122214708*r + 0.5363325363*g + 0.0514459929*b
	m := 0.2119034982*r + 0.6806995451*g + 0.1073969566*b
	s := 0.0883024619*r + 0.2817188376*g + 0.6299787005*b
	l = math.Cbrt(l)
	m = math.Cbrt(m)
	s = math.Cbrt(s)
	return 0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s
}

func srgbToLinear(value float64) float64 {
	if value <= 0.04045 {
		return value / 12.92
	}
	return math.Pow((value+0.055)/1.055, 2.4)
}
