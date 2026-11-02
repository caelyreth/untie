package font

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type mathTable struct {
	present       bool
	hasConstants  bool
	constants     [MathConstantCount]int32
	italic        map[GlyphID]int32
	topAccent     map[GlyphID]int32
	extendedShape map[GlyphID]struct{}
	kerns         map[GlyphID]*mathKernRecord
	minConnector  int32
	vertical      map[GlyphID]mathConstruction
	horizontal    map[GlyphID]mathConstruction
}

type mathKernRecord struct {
	corners [4]*KernCurve
}

type mathConstruction struct {
	variants []SizeVariant
	assembly *Assembly
}

func parseMathTable(b []byte) (mathTable, error) {
	var m mathTable
	if len(b) == 0 {
		return m, nil
	}
	if len(b) < 10 {
		return m, errors.New("MATH header is truncated")
	}
	major := binary.BigEndian.Uint16(b[0:2])
	if major != 1 {
		return m, fmt.Errorf("unsupported MATH major version %d", major)
	}
	m.present = true
	constantsOff := uint16(binary.BigEndian.Uint16(b[4:6]))
	glyphInfoOff := uint16(binary.BigEndian.Uint16(b[6:8]))
	variantsOff := uint16(binary.BigEndian.Uint16(b[8:10]))

	if constantsOff != 0 {
		sub, err := subtable(b, int(constantsOff))
		if err != nil {
			return m, fmt.Errorf("MATH constants: %w", err)
		}
		if err := m.parseConstants(sub); err != nil {
			return m, err
		}
	}
	if glyphInfoOff != 0 {
		sub, err := subtable(b, int(glyphInfoOff))
		if err != nil {
			return m, fmt.Errorf("MATH glyph info: %w", err)
		}
		if err := m.parseGlyphInfo(sub); err != nil {
			return m, err
		}
	}
	if variantsOff != 0 {
		sub, err := subtable(b, int(variantsOff))
		if err != nil {
			return m, fmt.Errorf("MATH variants: %w", err)
		}
		if err := m.parseVariants(sub); err != nil {
			return m, err
		}
	}
	return m, nil
}

func (m *mathTable) parseConstants(b []byte) error {
	const needed = 4 + (int(RadicalKernAfterDegree)-int(MathLeading)+1)*4 + 2
	if len(b) < needed {
		return fmt.Errorf("MATH constants are truncated: got %d bytes, need %d", len(b), needed)
	}
	off := 0
	m.constants[ScriptPercentScaleDown] = int32(readI16(b, &off))
	m.constants[ScriptScriptPercentScaleDown] = int32(readI16(b, &off))
	delimited, err := readU16(b, &off)
	if err != nil {
		return err
	}
	displayOperator, err := readU16(b, &off)
	if err != nil {
		return err
	}
	m.constants[DelimitedSubFormulaMinHeight] = int32(delimited)
	m.constants[DisplayOperatorMinHeight] = int32(displayOperator)
	for c := MathLeading; c <= RadicalKernAfterDegree; c++ {
		v, err := readMathValue(b, &off)
		if err != nil {
			return fmt.Errorf("MATH constant %s: %w", c, err)
		}
		m.constants[c] = int32(v)
	}
	m.constants[RadicalDegreeBottomRaisePercent] = int32(readI16(b, &off))
	m.hasConstants = true
	return nil
}

func (m *mathTable) parseGlyphInfo(b []byte) error {
	if len(b) < 8 {
		return errors.New("MATH glyph info header is truncated")
	}
	italicOff := int(binary.BigEndian.Uint16(b[0:2]))
	topAccentOff := int(binary.BigEndian.Uint16(b[2:4]))
	extendedOff := int(binary.BigEndian.Uint16(b[4:6]))
	kernOff := int(binary.BigEndian.Uint16(b[6:8]))
	if italicOff != 0 {
		sub, err := subtable(b, italicOff)
		if err != nil {
			return fmt.Errorf("MATH italic corrections: %w", err)
		}
		m.italic, err = parsePerGlyphMathValues(sub)
		if err != nil {
			return fmt.Errorf("MATH italic corrections: %w", err)
		}
	}
	if topAccentOff != 0 {
		sub, err := subtable(b, topAccentOff)
		if err != nil {
			return fmt.Errorf("MATH top accents: %w", err)
		}
		m.topAccent, err = parsePerGlyphMathValues(sub)
		if err != nil {
			return fmt.Errorf("MATH top accents: %w", err)
		}
	}
	if extendedOff != 0 {
		sub, err := subtable(b, extendedOff)
		if err != nil {
			return fmt.Errorf("MATH extended shapes: %w", err)
		}
		cov, err := parseCoverage(sub)
		if err != nil {
			return fmt.Errorf("MATH extended shapes: %w", err)
		}
		m.extendedShape = make(map[GlyphID]struct{}, len(cov))
		for gid := range cov {
			m.extendedShape[gid] = struct{}{}
		}
	}
	if kernOff != 0 {
		sub, err := subtable(b, kernOff)
		if err != nil {
			return fmt.Errorf("MATH kern info: %w", err)
		}
		m.kerns, err = parseMathKernInfo(sub)
		if err != nil {
			return fmt.Errorf("MATH kern info: %w", err)
		}
	}
	return nil
}

func (m *mathTable) parseVariants(b []byte) error {
	if len(b) < 10 {
		return errors.New("MATH variants header is truncated")
	}
	off := 0
	minConnector, err := readU16(b, &off)
	if err != nil {
		return err
	}
	m.minConnector = int32(minConnector)
	verticalCovOff := int(binary.BigEndian.Uint16(b[2:4]))
	horizontalCovOff := int(binary.BigEndian.Uint16(b[4:6]))
	verticalCount := int(binary.BigEndian.Uint16(b[6:8]))
	horizontalCount := int(binary.BigEndian.Uint16(b[8:10]))
	off = 10
	verticalOffsets, err := readU16s(b, &off, verticalCount)
	if err != nil {
		return fmt.Errorf("MATH vertical construction offsets: %w", err)
	}
	horizontalOffsets, err := readU16s(b, &off, horizontalCount)
	if err != nil {
		return fmt.Errorf("MATH horizontal construction offsets: %w", err)
	}

	var verticalCov, horizontalCov map[GlyphID]uint16
	if verticalCovOff != 0 {
		sub, err := subtable(b, verticalCovOff)
		if err != nil {
			return fmt.Errorf("MATH vertical coverage: %w", err)
		}
		verticalCov, err = parseCoverage(sub)
		if err != nil {
			return fmt.Errorf("MATH vertical coverage: %w", err)
		}
	}
	if horizontalCovOff != 0 {
		sub, err := subtable(b, horizontalCovOff)
		if err != nil {
			return fmt.Errorf("MATH horizontal coverage: %w", err)
		}
		horizontalCov, err = parseCoverage(sub)
		if err != nil {
			return fmt.Errorf("MATH horizontal coverage: %w", err)
		}
	}
	if len(verticalCov) != verticalCount {
		return fmt.Errorf("MATH vertical coverage has %d glyphs, want %d", len(verticalCov), verticalCount)
	}
	if len(horizontalCov) != horizontalCount {
		return fmt.Errorf("MATH horizontal coverage has %d glyphs, want %d", len(horizontalCov), horizontalCount)
	}

	verticalGlyphs, err := coverageGlyphs(verticalCov, verticalCount)
	if err != nil {
		return fmt.Errorf("MATH vertical coverage: %w", err)
	}
	horizontalGlyphs, err := coverageGlyphs(horizontalCov, horizontalCount)
	if err != nil {
		return fmt.Errorf("MATH horizontal coverage: %w", err)
	}
	m.vertical, err = m.parseConstructions(b, verticalGlyphs, verticalOffsets)
	if err != nil {
		return fmt.Errorf("MATH vertical constructions: %w", err)
	}
	m.horizontal, err = m.parseConstructions(b, horizontalGlyphs, horizontalOffsets)
	if err != nil {
		return fmt.Errorf("MATH horizontal constructions: %w", err)
	}
	return nil
}

func coverageGlyphs(cov map[GlyphID]uint16, count int) ([]GlyphID, error) {
	glyphs := make([]GlyphID, count)
	filled := make([]bool, count)
	for gid, idx := range cov {
		if int(idx) >= count {
			return nil, fmt.Errorf("coverage index %d is out of range", idx)
		}
		if filled[idx] {
			return nil, fmt.Errorf("duplicate coverage index %d", idx)
		}
		glyphs[idx] = gid
		filled[idx] = true
	}
	return glyphs, nil
}

func (m *mathTable) parseConstructions(b []byte, glyphs []GlyphID, offsets []uint16) (map[GlyphID]mathConstruction, error) {
	out := make(map[GlyphID]mathConstruction, len(offsets))
	for i, rel := range offsets {
		if rel == 0 {
			return nil, fmt.Errorf("construction %d has zero offset", i)
		}
		sub, err := subtable(b, int(rel))
		if err != nil {
			return nil, fmt.Errorf("construction %d: %w", i, err)
		}
		c, err := parseConstruction(sub, m.minConnector)
		if err != nil {
			return nil, fmt.Errorf("construction %d: %w", i, err)
		}
		if i >= len(glyphs) {
			return nil, fmt.Errorf("construction %d has no covered glyph", i)
		}
		out[glyphs[i]] = c
	}
	return out, nil
}

func parseConstruction(b []byte, minConnector int32) (mathConstruction, error) {
	if len(b) < 4 {
		return mathConstruction{}, errors.New("MathGlyphConstruction is truncated")
	}
	assemblyOff := int(binary.BigEndian.Uint16(b[0:2]))
	count := int(binary.BigEndian.Uint16(b[2:4]))
	if count > (len(b)-4)/4 {
		return mathConstruction{}, errors.New("MathGlyphConstruction variants are truncated")
	}
	off := 4
	variants := make([]SizeVariant, count)
	for i := range variants {
		gid, err := readU16(b, &off)
		if err != nil {
			return mathConstruction{}, err
		}
		advance, err := readU16(b, &off)
		if err != nil {
			return mathConstruction{}, err
		}
		variants[i] = SizeVariant{Glyph: GlyphID(gid), Advance: int32(advance)}
	}
	var assembly *Assembly
	if assemblyOff != 0 {
		sub, err := subtable(b, assemblyOff)
		if err != nil {
			return mathConstruction{}, fmt.Errorf("GlyphAssembly: %w", err)
		}
		a, err := parseAssembly(sub, minConnector)
		if err != nil {
			return mathConstruction{}, err
		}
		assembly = &a
	}
	return mathConstruction{variants: variants, assembly: assembly}, nil
}

func parseAssembly(b []byte, minConnector int32) (Assembly, error) {
	if len(b) < 6 {
		return Assembly{}, errors.New("GlyphAssembly is truncated")
	}
	off := 0
	italic, err := readMathValue(b, &off)
	if err != nil {
		return Assembly{}, err
	}
	count16, err := readU16(b, &off)
	if err != nil {
		return Assembly{}, err
	}
	count := int(count16)
	if count == 0 {
		return Assembly{}, errors.New("GlyphAssembly has no parts")
	}
	if count > (len(b)-off)/10 {
		return Assembly{}, errors.New("GlyphAssembly parts are truncated")
	}
	parts := make([]AssemblyPart, count)
	for i := range parts {
		gid, err := readU16(b, &off)
		if err != nil {
			return Assembly{}, err
		}
		start, err := readU16(b, &off)
		if err != nil {
			return Assembly{}, err
		}
		end, err := readU16(b, &off)
		if err != nil {
			return Assembly{}, err
		}
		full, err := readU16(b, &off)
		if err != nil {
			return Assembly{}, err
		}
		flags, err := readU16(b, &off)
		if err != nil {
			return Assembly{}, err
		}
		if flags&^uint16(1) != 0 {
			return Assembly{}, fmt.Errorf("GlyphPart %d has unsupported flags 0x%04x", i, flags)
		}
		parts[i] = AssemblyPart{
			Glyph:          GlyphID(gid),
			StartConnector: int32(start),
			EndConnector:   int32(end),
			FullAdvance:    int32(full),
			Extender:       flags&1 != 0,
		}
	}
	return Assembly{ItalicCorrection: int32(italic), MinConnectorOverlap: minConnector, Parts: parts}, nil
}

func (m mathTable) validateGlyphIDs(glyphCount int) error {
	if glyphCount <= 0 {
		return errors.New("MATH requires a positive glyph count")
	}
	valid := func(gid GlyphID) bool { return int(gid) < glyphCount }
	checkValues := func(name string, values map[GlyphID]int32) error {
		for gid := range values {
			if !valid(gid) {
				return fmt.Errorf("%s glyph %d is outside %d glyphs", name, gid, glyphCount)
			}
		}
		return nil
	}
	if err := checkValues("italic correction", m.italic); err != nil {
		return err
	}
	if err := checkValues("top accent", m.topAccent); err != nil {
		return err
	}
	for gid := range m.kerns {
		if !valid(gid) {
			return fmt.Errorf("kern glyph %d is outside %d glyphs", gid, glyphCount)
		}
	}
	for gid := range m.extendedShape {
		if !valid(gid) {
			return fmt.Errorf("extended shape glyph %d is outside %d glyphs", gid, glyphCount)
		}
	}
	for axis, table := range map[MathAxis]map[GlyphID]mathConstruction{
		VerticalAxis:   m.vertical,
		HorizontalAxis: m.horizontal,
	} {
		for gid, construction := range table {
			if !valid(gid) {
				return fmt.Errorf("axis %d construction glyph %d is outside %d glyphs", axis, gid, glyphCount)
			}
			for i, variant := range construction.variants {
				if !valid(variant.Glyph) {
					return fmt.Errorf("axis %d glyph %d variant %d glyph %d is outside %d glyphs", axis, gid, i, variant.Glyph, glyphCount)
				}
			}
			if construction.assembly == nil {
				continue
			}
			for i, part := range construction.assembly.Parts {
				if !valid(part.Glyph) {
					return fmt.Errorf("axis %d glyph %d assembly part %d glyph %d is outside %d glyphs", axis, gid, i, part.Glyph, glyphCount)
				}
			}
		}
	}
	return nil
}

func parsePerGlyphMathValues(b []byte) (map[GlyphID]int32, error) {
	if len(b) < 4 {
		return nil, errors.New("per-glyph MATH values are truncated")
	}
	covOff := int(binary.BigEndian.Uint16(b[0:2]))
	count := int(binary.BigEndian.Uint16(b[2:4]))
	covBytes, err := subtable(b, covOff)
	if err != nil {
		return nil, err
	}
	cov, err := parseCoverage(covBytes)
	if err != nil {
		return nil, err
	}
	if len(cov) != count {
		return nil, fmt.Errorf("coverage has %d glyphs, value count is %d", len(cov), count)
	}
	if count > (len(b)-4)/4 {
		return nil, errors.New("per-glyph MATH values are truncated")
	}
	off := 4
	values := make([]int32, count)
	for i := range values {
		v, err := readMathValue(b, &off)
		if err != nil {
			return nil, err
		}
		values[i] = int32(v)
	}
	out := make(map[GlyphID]int32, count)
	for gid, idx := range cov {
		out[gid] = values[idx]
	}
	return out, nil
}

func parseMathKernInfo(b []byte) (map[GlyphID]*mathKernRecord, error) {
	if len(b) < 2 {
		return nil, errors.New("MathKernInfo is truncated")
	}
	covOff := int(binary.BigEndian.Uint16(b[0:2]))
	covBytes, err := subtable(b, covOff)
	if err != nil {
		return nil, err
	}
	cov, err := parseCoverage(covBytes)
	if err != nil {
		return nil, err
	}
	if len(cov) > (len(b)-2)/8 {
		return nil, errors.New("MathKernInfo records are truncated")
	}
	off := 2
	records := make([][4]uint16, len(cov))
	for i := range records {
		for corner := range records[i] {
			v, err := readU16(b, &off)
			if err != nil {
				return nil, err
			}
			records[i][corner] = v
		}
	}
	out := make(map[GlyphID]*mathKernRecord, len(cov))
	curves := make(map[uint16]*KernCurve)
	// Typst/read-fonts exposes the first and third stored records as the
	// top-left and bottom-left cut-ins. Use that side mapping so callers see the
	// same semantic corners as the pinned layout oracle.
	cornerOrder := [...]MathKernCorner{
		MathKernTopLeft,
		MathKernTopRight,
		MathKernBottomLeft,
		MathKernBottomRight,
	}
	for gid, idx := range cov {
		rec := &mathKernRecord{}
		has := false
		for field, rel := range records[idx] {
			if rel == 0 {
				continue
			}
			curve := curves[rel]
			if curve == nil {
				sub, err := subtable(b, int(rel))
				if err != nil {
					return nil, err
				}
				parsed, err := parseKernCurve(sub)
				if err != nil {
					return nil, err
				}
				curve = &parsed
				curves[rel] = curve
			}
			corner := cornerOrder[field]
			rec.corners[corner] = curve
			has = true
		}
		if has {
			out[gid] = rec
		}
	}
	return out, nil
}

func parseKernCurve(b []byte) (KernCurve, error) {
	if len(b) < 2 {
		return KernCurve{}, errors.New("MathKern is truncated")
	}
	off := 0
	count16, err := readU16(b, &off)
	if err != nil {
		return KernCurve{}, err
	}
	count := int(count16)
	if 2*count+1 > (len(b)-off)/4 {
		return KernCurve{}, errors.New("MathKern values are truncated")
	}
	heights := make([]int32, count)
	for i := range heights {
		v, err := readMathValue(b, &off)
		if err != nil {
			return KernCurve{}, err
		}
		heights[i] = int32(v)
		if i > 0 && heights[i] <= heights[i-1] {
			return KernCurve{}, errors.New("MathKern heights are not strictly ascending")
		}
	}
	kerns := make([]int32, count+1)
	for i := range kerns {
		v, err := readMathValue(b, &off)
		if err != nil {
			return KernCurve{}, err
		}
		kerns[i] = int32(v)
	}
	return KernCurve{Heights: heights, Kerns: kerns}, nil
}

func parseCoverage(b []byte) (map[GlyphID]uint16, error) {
	if len(b) < 4 {
		return nil, errors.New("coverage table is truncated")
	}
	format := binary.BigEndian.Uint16(b[0:2])
	count := int(binary.BigEndian.Uint16(b[2:4]))
	out := make(map[GlyphID]uint16)
	off := 4
	switch format {
	case 1:
		for i := 0; i < count; i++ {
			gid, err := readU16(b, &off)
			if err != nil {
				return nil, err
			}
			if _, exists := out[GlyphID(gid)]; exists {
				return nil, fmt.Errorf("duplicate glyph %d in coverage", gid)
			}
			out[GlyphID(gid)] = uint16(i)
		}
	case 2:
		var next uint32
		for i := 0; i < count; i++ {
			start, err := readU16(b, &off)
			if err != nil {
				return nil, err
			}
			end, err := readU16(b, &off)
			if err != nil {
				return nil, err
			}
			startCoverage, err := readU16(b, &off)
			if err != nil {
				return nil, err
			}
			if start > end {
				return nil, fmt.Errorf("coverage range starts at %d after ending at %d", start, end)
			}
			if uint32(startCoverage) != next {
				return nil, fmt.Errorf("coverage range starts at index %d, want %d", startCoverage, next)
			}
			for gid := uint32(start); gid <= uint32(end); gid++ {
				if next > 0xffff {
					return nil, errors.New("coverage index exceeds uint16")
				}
				if _, exists := out[GlyphID(gid)]; exists {
					return nil, fmt.Errorf("duplicate glyph %d in coverage", gid)
				}
				out[GlyphID(gid)] = uint16(next)
				next++
			}
		}
	default:
		return nil, fmt.Errorf("unsupported coverage format %d", format)
	}
	return out, nil
}

func readMathValue(b []byte, off *int) (int16, error) {
	if *off < 0 || *off+4 > len(b) {
		return 0, errors.New("unexpected end of table")
	}
	v := readI16(b, off)
	*off += 2 // Device table offset; ignored for scalable layout.
	return v, nil
}

func readU16(b []byte, off *int) (uint16, error) {
	if *off < 0 || *off+2 > len(b) {
		return 0, errors.New("unexpected end of table")
	}
	v := binary.BigEndian.Uint16(b[*off : *off+2])
	*off += 2
	return v, nil
}

func readU16s(b []byte, off *int, n int) ([]uint16, error) {
	if n < 0 {
		return nil, errors.New("negative array length")
	}
	out := make([]uint16, n)
	for i := range out {
		v, err := readU16(b, off)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func readI16(b []byte, off *int) int16 {
	if *off < 0 || *off+2 > len(b) {
		*off = len(b) + 1
		return 0
	}
	v := int16(binary.BigEndian.Uint16(b[*off : *off+2]))
	*off += 2
	return v
}

func subtable(b []byte, off int) ([]byte, error) {
	if off <= 0 || off >= len(b) {
		return nil, fmt.Errorf("offset %d outside %d-byte table", off, len(b))
	}
	return b[off:], nil
}
