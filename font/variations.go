package font

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"

	tsfont "github.com/go-text/typesetting/font"
)

// Variation selects a value on one OpenType design-variation axis.
//
// Values are expressed in the font's design coordinate space, not normalized
// CSS coordinates. Tag must be an exact four-byte OpenType axis tag such as
// "wght" or "wdth".
type Variation struct {
	Tag   string
	Value float32
}

// VariationAxis describes one axis declared by a face's OpenType fvar table.
type VariationAxis struct {
	Tag     string
	Minimum float32
	Default float32
	Maximum float32
}

type resolvedVariation struct {
	axes         []VariationAxis
	designValues []float32
	coordinates  []tsfont.VarCoord
}

func parseVariationAxes(table []byte) ([]VariationAxis, error) {
	if len(table) == 0 {
		return nil, nil
	}
	if len(table) < 16 {
		return nil, errors.New("fvar header is truncated")
	}
	major := binary.BigEndian.Uint16(table[0:2])
	minor := binary.BigEndian.Uint16(table[2:4])
	if major != 1 || minor != 0 {
		return nil, fmt.Errorf("unsupported fvar version %d.%d", major, minor)
	}
	axesOffset := int(binary.BigEndian.Uint16(table[4:6]))
	axisCount := int(binary.BigEndian.Uint16(table[8:10]))
	axisSize := int(binary.BigEndian.Uint16(table[10:12]))
	if axisCount == 0 {
		return nil, errors.New("fvar declares no axes")
	}
	if axisSize < 20 {
		return nil, fmt.Errorf("fvar axis record size %d is smaller than 20", axisSize)
	}
	if axesOffset < 16 || axesOffset > len(table) || axisCount > (len(table)-axesOffset)/axisSize {
		return nil, errors.New("fvar axes are truncated")
	}
	axes := make([]VariationAxis, axisCount)
	seen := make(map[string]struct{}, axisCount)
	for i := range axes {
		start := axesOffset + i*axisSize
		tag := string(table[start : start+4])
		if err := validateVariationTag(tag); err != nil {
			return nil, fmt.Errorf("fvar axis %d: %w", i, err)
		}
		if _, exists := seen[tag]; exists {
			return nil, fmt.Errorf("fvar contains duplicate axis tag %q", tag)
		}
		seen[tag] = struct{}{}
		minimum := fixed1616(table[start+4 : start+8])
		defaultValue := fixed1616(table[start+8 : start+12])
		maximum := fixed1616(table[start+12 : start+16])
		if minimum > defaultValue || defaultValue > maximum {
			return nil, fmt.Errorf(
				"fvar axis %q has invalid range [%g,%g,%g]",
				tag,
				minimum,
				defaultValue,
				maximum,
			)
		}
		axes[i] = VariationAxis{
			Tag:     tag,
			Minimum: minimum,
			Default: defaultValue,
			Maximum: maximum,
		}
	}
	return axes, nil
}

func resolveVariations(
	font *tsfont.Font,
	axes []VariationAxis,
	requested []Variation,
) (resolvedVariation, error) {
	if len(requested) == 0 {
		if len(axes) == 0 {
			return resolvedVariation{}, nil
		}
		design := make([]float32, len(axes))
		for i, axis := range axes {
			design[i] = axis.Default
		}
		return resolvedVariation{
			axes:         append([]VariationAxis(nil), axes...),
			designValues: design,
			coordinates:  font.NormalizeVariations(design),
		}, nil
	}
	if len(axes) == 0 {
		return resolvedVariation{}, errors.New("variation coordinates were supplied for a static face")
	}

	design := make([]float32, len(axes))
	index := make(map[string]int, len(axes))
	for i, axis := range axes {
		design[i] = axis.Default
		index[axis.Tag] = i
	}
	seen := make(map[string]struct{}, len(requested))
	for i, variation := range requested {
		if err := validateVariationTag(variation.Tag); err != nil {
			return resolvedVariation{}, fmt.Errorf("variation %d: %w", i, err)
		}
		if _, exists := seen[variation.Tag]; exists {
			return resolvedVariation{}, fmt.Errorf("duplicate variation axis %q", variation.Tag)
		}
		seen[variation.Tag] = struct{}{}
		axisIndex, ok := index[variation.Tag]
		if !ok {
			return resolvedVariation{}, fmt.Errorf("unknown variation axis %q", variation.Tag)
		}
		if math.IsNaN(float64(variation.Value)) || math.IsInf(float64(variation.Value), 0) {
			return resolvedVariation{}, fmt.Errorf("variation axis %q has non-finite value", variation.Tag)
		}
		axis := axes[axisIndex]
		if variation.Value < axis.Minimum || variation.Value > axis.Maximum {
			return resolvedVariation{}, fmt.Errorf(
				"variation axis %q value %g is outside [%g,%g]",
				variation.Tag,
				variation.Value,
				axis.Minimum,
				axis.Maximum,
			)
		}
		design[axisIndex] = variation.Value
	}
	return resolvedVariation{
		axes:         append([]VariationAxis(nil), axes...),
		designValues: design,
		coordinates:  font.NormalizeVariations(design),
	}, nil
}

func validateVariationTag(tag string) error {
	if len(tag) != 4 {
		return fmt.Errorf("axis tag %q must contain exactly four bytes", tag)
	}
	for i := 0; i < len(tag); i++ {
		if tag[i] < 0x20 || tag[i] > 0x7e {
			return fmt.Errorf("axis tag %q contains a non-printable byte", tag)
		}
	}
	return nil
}

func fixed1616(b []byte) float32 {
	return float32(int32(binary.BigEndian.Uint32(b))) / 65536
}

func sortedVariations(variations []Variation) []Variation {
	if len(variations) == 0 {
		return nil
	}
	out := append([]Variation(nil), variations...)
	for i := range out {
		if out[i].Value == 0 {
			out[i].Value = 0
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Tag < out[j].Tag
	})
	return out
}

func normalizedCoordinateValues(coords []tsfont.VarCoord) []float32 {
	if len(coords) == 0 {
		return nil
	}
	out := make([]float32, len(coords))
	for i, coord := range coords {
		out[i] = float32(coord) / 16384
	}
	return out
}
