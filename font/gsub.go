package font

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

// The math substrate only needs the one-to-one GSUB operations used by the
// standard math features ssty and dtls. Parsing this narrow surface locally
// preserves alternate-feature values (ssty=1 and ssty=2), which a generic
// first-alternate API cannot express.

var mathFeatureTags = map[string]struct{}{
	"ssty": {},
	"dtls": {},
}

type gsubTable struct {
	present  bool
	features map[string][]gsubLookup
}

type gsubLookup struct {
	subtables []gsubSubtable
}

type gsubSubtable struct {
	single     map[GlyphID]GlyphID
	alternates map[GlyphID][]GlyphID
}

func parseGSUBTable(b []byte, coordinates []float32) (gsubTable, error) {
	g := gsubTable{}
	if len(b) == 0 {
		return g, nil
	}
	if len(b) < 10 {
		return g, errors.New("GSUB header is truncated")
	}
	major := binary.BigEndian.Uint16(b[0:2])
	minor := binary.BigEndian.Uint16(b[2:4])
	if major != 1 || minor > 1 {
		return g, fmt.Errorf("unsupported GSUB version %d.%d", major, minor)
	}
	if minor == 1 && len(b) < 14 {
		return g, errors.New("GSUB 1.1 header is truncated")
	}
	featureOff := int(binary.BigEndian.Uint16(b[6:8]))
	lookupOff := int(binary.BigEndian.Uint16(b[8:10]))
	featureBytes, err := subtableAllowHeader(b, featureOff)
	if err != nil {
		return g, fmt.Errorf("FeatureList: %w", err)
	}
	lookupBytes, err := subtableAllowHeader(b, lookupOff)
	if err != nil {
		return g, fmt.Errorf("LookupList: %w", err)
	}

	records, err := parseFeatureRecords(featureBytes)
	if err != nil {
		return g, err
	}
	activeLookups := make(map[int][]uint16, len(records))
	for _, rec := range records {
		activeLookups[rec.index] = rec.lookups
	}
	if minor == 1 && len(b) >= 14 {
		featureVariationsOff := int(binary.BigEndian.Uint32(b[10:14]))
		if featureVariationsOff != 0 {
			substitutions, err := parseFeatureVariations(
				b,
				featureVariationsOff,
				len(records),
				coordinates,
			)
			if err != nil {
				return g, fmt.Errorf("FeatureVariations: %w", err)
			}
			for index, lookups := range substitutions {
				activeLookups[index] = lookups
			}
		}
	}
	lookupOffsets, err := parseLookupOffsets(lookupBytes)
	if err != nil {
		return g, err
	}
	needed := make(map[int]struct{})
	for _, rec := range records {
		if _, ok := mathFeatureTags[rec.tag]; !ok {
			continue
		}
		for _, idx := range activeLookups[rec.index] {
			if int(idx) >= len(lookupOffsets) {
				return g, fmt.Errorf("feature %q references lookup %d of %d", rec.tag, idx, len(lookupOffsets))
			}
			needed[int(idx)] = struct{}{}
		}
	}
	lookups := make(map[int]gsubLookup, len(needed))
	for idx := range needed {
		l, err := parseGSUBLookup(lookupBytes, lookupOffsets[idx])
		if err != nil {
			return g, fmt.Errorf("lookup %d: %w", idx, err)
		}
		lookups[idx] = l
	}

	g.present = true
	g.features = make(map[string][]gsubLookup)
	for _, rec := range records {
		if _, ok := mathFeatureTags[rec.tag]; !ok {
			continue
		}
		for _, idx := range activeLookups[rec.index] {
			g.features[rec.tag] = append(g.features[rec.tag], lookups[int(idx)])
		}
	}
	return g, nil
}

type featureRecord struct {
	index   int
	tag     string
	lookups []uint16
}

func parseFeatureRecords(b []byte) ([]featureRecord, error) {
	if len(b) < 2 {
		return nil, errors.New("FeatureList is truncated")
	}
	count := int(binary.BigEndian.Uint16(b[0:2]))
	off := 2
	records := make([]featureRecord, count)
	for i := range records {
		if off+6 > len(b) {
			return nil, errors.New("FeatureList records are truncated")
		}
		records[i].index = i
		records[i].tag = string(b[off : off+4])
		featureOff := int(binary.BigEndian.Uint16(b[off+4 : off+6]))
		off += 6
		if featureOff <= 0 || featureOff >= len(b) {
			return nil, fmt.Errorf("feature %q has invalid offset %d", records[i].tag, featureOff)
		}
		f := b[featureOff:]
		if len(f) < 4 {
			return nil, fmt.Errorf("feature %q is truncated", records[i].tag)
		}
		featureOff = 0
		_, err := readU16(f, &featureOff)
		if err != nil {
			return nil, err
		}
		lookupCount, err := readU16(f, &featureOff)
		if err != nil {
			return nil, err
		}
		if int(lookupCount) > (len(f)-featureOff)/2 {
			return nil, fmt.Errorf("feature %q lookup list is truncated", records[i].tag)
		}
		lookups := make([]uint16, int(lookupCount))
		for j := range lookups {
			lookups[j], err = readU16(f, &featureOff)
			if err != nil {
				return nil, err
			}
		}
		records[i].lookups = lookups
	}
	return records, nil
}

func parseFeatureVariations(
	table []byte,
	offset int,
	featureCount int,
	coordinates []float32,
) (map[int][]uint16, error) {
	if offset <= 0 || offset > len(table) {
		return nil, fmt.Errorf("offset %d is outside %d-byte table", offset, len(table))
	}
	b := table[offset:]
	if len(b) < 8 {
		return nil, errors.New("header is truncated")
	}
	major := binary.BigEndian.Uint16(b[0:2])
	minor := binary.BigEndian.Uint16(b[2:4])
	if major != 1 || minor != 0 {
		return nil, fmt.Errorf("unsupported version %d.%d", major, minor)
	}
	count64 := uint64(binary.BigEndian.Uint32(b[4:8]))
	if count64 > uint64((len(b)-8)/8) {
		return nil, errors.New("records are truncated")
	}
	count := int(count64)
	for i := 0; i < count; i++ {
		record := b[8+i*8 : 16+i*8]
		conditionOff := int(binary.BigEndian.Uint32(record[0:4]))
		substitutionOff := int(binary.BigEndian.Uint32(record[4:8]))
		matches, err := featureConditionsMatch(b, conditionOff, coordinates)
		if err != nil {
			return nil, fmt.Errorf("record %d conditions: %w", i, err)
		}
		if !matches {
			continue
		}
		substitutions, err := parseFeatureTableSubstitution(b, substitutionOff, featureCount)
		if err != nil {
			return nil, fmt.Errorf("record %d substitution: %w", i, err)
		}
		return substitutions, nil
	}
	return nil, nil
}

func featureConditionsMatch(
	table []byte,
	offset int,
	coordinates []float32,
) (bool, error) {
	if offset == 0 {
		return true, nil
	}
	if offset < 0 || offset > len(table) {
		return false, fmt.Errorf("offset %d is outside %d-byte table", offset, len(table))
	}
	b := table[offset:]
	if len(b) < 2 {
		return false, errors.New("condition set is truncated")
	}
	count := int(binary.BigEndian.Uint16(b[0:2]))
	if count > (len(b)-2)/4 {
		return false, errors.New("condition offsets are truncated")
	}
	for i := 0; i < count; i++ {
		conditionOff := int(binary.BigEndian.Uint32(b[2+i*4 : 6+i*4]))
		if conditionOff <= 0 || conditionOff > len(b) {
			return false, fmt.Errorf("condition %d offset %d is outside %d-byte set", i, conditionOff, len(b))
		}
		condition := b[conditionOff:]
		if len(condition) < 8 {
			return false, fmt.Errorf("condition %d is truncated", i)
		}
		format := binary.BigEndian.Uint16(condition[0:2])
		if format != 1 {
			return false, fmt.Errorf("condition %d has unsupported format %d", i, format)
		}
		axisIndex := int(binary.BigEndian.Uint16(condition[2:4]))
		if axisIndex >= len(coordinates) {
			return false, fmt.Errorf("condition %d references axis %d of %d", i, axisIndex, len(coordinates))
		}
		minimum := float32(int16(binary.BigEndian.Uint16(condition[4:6]))) / 16384
		maximum := float32(int16(binary.BigEndian.Uint16(condition[6:8]))) / 16384
		if minimum > maximum || minimum < -1 || maximum > 1 {
			return false, fmt.Errorf("condition %d has invalid range [%g,%g]", i, minimum, maximum)
		}
		value := coordinates[axisIndex]
		if value < minimum || value > maximum {
			return false, nil
		}
	}
	return true, nil
}

func parseFeatureTableSubstitution(
	table []byte,
	offset int,
	featureCount int,
) (map[int][]uint16, error) {
	if offset <= 0 || offset > len(table) {
		return nil, fmt.Errorf("offset %d is outside %d-byte table", offset, len(table))
	}
	b := table[offset:]
	if len(b) < 6 {
		return nil, errors.New("header is truncated")
	}
	major := binary.BigEndian.Uint16(b[0:2])
	minor := binary.BigEndian.Uint16(b[2:4])
	if major != 1 || minor != 0 {
		return nil, fmt.Errorf("unsupported version %d.%d", major, minor)
	}
	count := int(binary.BigEndian.Uint16(b[4:6]))
	if count > (len(b)-6)/6 {
		return nil, errors.New("records are truncated")
	}
	out := make(map[int][]uint16, count)
	for i := 0; i < count; i++ {
		record := b[6+i*6 : 12+i*6]
		featureIndex := int(binary.BigEndian.Uint16(record[0:2]))
		if featureIndex >= featureCount {
			return nil, fmt.Errorf("feature index %d is outside %d features", featureIndex, featureCount)
		}
		if _, exists := out[featureIndex]; exists {
			return nil, fmt.Errorf("duplicate feature index %d", featureIndex)
		}
		alternateOff := int(binary.BigEndian.Uint32(record[2:6]))
		if alternateOff <= 0 || alternateOff > len(b) {
			return nil, fmt.Errorf("alternate feature %d offset %d is outside %d-byte table", featureIndex, alternateOff, len(b))
		}
		alternate := b[alternateOff:]
		if len(alternate) < 4 {
			return nil, fmt.Errorf("alternate feature %d is truncated", featureIndex)
		}
		lookupCount := int(binary.BigEndian.Uint16(alternate[2:4]))
		if lookupCount > (len(alternate)-4)/2 {
			return nil, fmt.Errorf("alternate feature %d lookup list is truncated", featureIndex)
		}
		lookups := make([]uint16, lookupCount)
		for j := range lookups {
			lookups[j] = binary.BigEndian.Uint16(alternate[4+j*2 : 6+j*2])
		}
		out[featureIndex] = lookups
	}
	return out, nil
}

func parseLookupOffsets(b []byte) ([]uint16, error) {
	if len(b) < 2 {
		return nil, errors.New("LookupList is truncated")
	}
	count := int(binary.BigEndian.Uint16(b[0:2]))
	if count > (len(b)-2)/2 {
		return nil, errors.New("LookupList offsets are truncated")
	}
	off := 2
	return readU16s(b, &off, count)
}

func parseGSUBLookup(list []byte, rel uint16) (gsubLookup, error) {
	if rel == 0 || int(rel) >= len(list) {
		return gsubLookup{}, fmt.Errorf("invalid lookup offset %d", rel)
	}
	b := list[rel:]
	if len(b) < 6 {
		return gsubLookup{}, errors.New("lookup header is truncated")
	}
	lookupType := binary.BigEndian.Uint16(b[0:2])
	flag := binary.BigEndian.Uint16(b[2:4])
	count := int(binary.BigEndian.Uint16(b[4:6]))
	if flag != 0 {
		return gsubLookup{}, fmt.Errorf("math feature lookup flags 0x%04x are unsupported", flag)
	}
	if count > (len(b)-6)/2 {
		return gsubLookup{}, errors.New("lookup subtable offsets are truncated")
	}
	off := 6
	offsets, err := readU16s(b, &off, count)
	if err != nil {
		return gsubLookup{}, err
	}
	lookup := gsubLookup{subtables: make([]gsubSubtable, 0, count)}
	for _, subOff := range offsets {
		if subOff == 0 || int(subOff) >= len(b) {
			return gsubLookup{}, fmt.Errorf("invalid subtable offset %d", subOff)
		}
		st, err := parseGSUBSubtable(b[subOff:], lookupType)
		if err != nil {
			return gsubLookup{}, err
		}
		lookup.subtables = append(lookup.subtables, st)
	}
	return lookup, nil
}

func parseGSUBSubtable(b []byte, lookupType uint16) (gsubSubtable, error) {
	if len(b) < 2 {
		return gsubSubtable{}, errors.New("subtable is truncated")
	}
	format := binary.BigEndian.Uint16(b[0:2])
	if lookupType == 7 {
		if format != 1 || len(b) < 8 {
			return gsubSubtable{}, errors.New("unsupported GSUB extension subtable")
		}
		nestedType := binary.BigEndian.Uint16(b[2:4])
		nestedOff := binary.BigEndian.Uint32(b[4:8])
		if nestedOff == 0 || nestedOff >= uint32(len(b)) {
			return gsubSubtable{}, fmt.Errorf("invalid extension offset %d", nestedOff)
		}
		return parseGSUBSubtable(b[nestedOff:], nestedType)
	}

	switch lookupType {
	case 1:
		return parseSingleSubstitution(b, format)
	case 3:
		return parseAlternateSubstitution(b, format)
	default:
		return gsubSubtable{}, fmt.Errorf("unsupported math feature lookup type %d", lookupType)
	}
}

func parseSingleSubstitution(b []byte, format uint16) (gsubSubtable, error) {
	if len(b) < 6 {
		return gsubSubtable{}, errors.New("single substitution is truncated")
	}
	covOff := int(binary.BigEndian.Uint16(b[2:4]))
	covBytes, err := subtableAllowHeader(b, covOff)
	if err != nil {
		return gsubSubtable{}, err
	}
	cov, err := parseCoverage(covBytes)
	if err != nil {
		return gsubSubtable{}, err
	}
	single := make(map[GlyphID]GlyphID, len(cov))
	switch format {
	case 1:
		delta := int16(binary.BigEndian.Uint16(b[4:6]))
		for gid := range cov {
			replacement := int32(gid) + int32(delta)
			if replacement < 0 || replacement > 0xffff {
				return gsubSubtable{}, fmt.Errorf("single substitution delta moves glyph %d out of range", gid)
			}
			single[gid] = GlyphID(replacement)
		}
	case 2:
		count := int(binary.BigEndian.Uint16(b[4:6]))
		off := 6
		substitutes, err := readU16s(b, &off, count)
		if err != nil {
			return gsubSubtable{}, err
		}
		if len(cov) != count {
			return gsubSubtable{}, fmt.Errorf("single substitution coverage has %d glyphs, count is %d", len(cov), count)
		}
		for gid, idx := range cov {
			single[gid] = GlyphID(substitutes[idx])
		}
	default:
		return gsubSubtable{}, fmt.Errorf("unsupported single substitution format %d", format)
	}
	return gsubSubtable{single: single}, nil
}

func parseAlternateSubstitution(b []byte, format uint16) (gsubSubtable, error) {
	if format != 1 || len(b) < 6 {
		return gsubSubtable{}, fmt.Errorf("unsupported alternate substitution format %d", format)
	}
	covOff := int(binary.BigEndian.Uint16(b[2:4]))
	covBytes, err := subtableAllowHeader(b, covOff)
	if err != nil {
		return gsubSubtable{}, err
	}
	cov, err := parseCoverage(covBytes)
	if err != nil {
		return gsubSubtable{}, err
	}
	count := int(binary.BigEndian.Uint16(b[4:6]))
	if len(cov) != count {
		return gsubSubtable{}, fmt.Errorf("alternate substitution coverage has %d glyphs, count is %d", len(cov), count)
	}
	off := 6
	setOffsets, err := readU16s(b, &off, count)
	if err != nil {
		return gsubSubtable{}, err
	}
	alternates := make(map[GlyphID][]GlyphID, len(cov))
	for gid, idx := range cov {
		setOff := int(setOffsets[idx])
		setBytes, err := subtableAllowHeader(b, setOff)
		if err != nil {
			return gsubSubtable{}, err
		}
		setOff = 0
		n, err := readU16(setBytes, &setOff)
		if err != nil {
			return gsubSubtable{}, err
		}
		glyphs, err := readU16s(setBytes, &setOff, int(n))
		if err != nil {
			return gsubSubtable{}, err
		}
		if len(glyphs) == 0 {
			return gsubSubtable{}, fmt.Errorf("alternate substitution glyph %d has no alternates", gid)
		}
		alt := make([]GlyphID, len(glyphs))
		for i := range glyphs {
			alt[i] = GlyphID(glyphs[i])
		}
		alternates[gid] = alt
	}
	return gsubSubtable{alternates: alternates}, nil
}

func (g gsubTable) featureTags() []string {
	if len(g.features) == 0 {
		return nil
	}
	tags := make([]string, 0, len(g.features))
	for tag := range g.features {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func (g gsubTable) validateGlyphIDs(glyphCount int) error {
	if glyphCount <= 0 {
		return errors.New("GSUB requires a positive glyph count")
	}
	valid := func(gid GlyphID) bool { return int(gid) < glyphCount }
	for tag, lookups := range g.features {
		for lookupIndex, lookup := range lookups {
			for subtableIndex, st := range lookup.subtables {
				for source, replacement := range st.single {
					if !valid(source) || !valid(replacement) {
						return fmt.Errorf("feature %q lookup %d subtable %d single substitution %d->%d is outside %d glyphs", tag, lookupIndex, subtableIndex, source, replacement, glyphCount)
					}
				}
				for source, alternates := range st.alternates {
					if !valid(source) {
						return fmt.Errorf("feature %q lookup %d subtable %d source glyph %d is outside %d glyphs", tag, lookupIndex, subtableIndex, source, glyphCount)
					}
					for alternateIndex, replacement := range alternates {
						if !valid(replacement) {
							return fmt.Errorf("feature %q lookup %d subtable %d alternate %d glyph %d is outside %d glyphs", tag, lookupIndex, subtableIndex, alternateIndex, replacement, glyphCount)
						}
					}
				}
			}
		}
	}
	return nil
}

func (g gsubTable) shape(glyphs []GlyphID, features []Feature) []GlyphID {
	if len(glyphs) == 0 || len(features) == 0 || len(g.features) == 0 {
		return append([]GlyphID(nil), glyphs...)
	}
	out := append([]GlyphID(nil), glyphs...)
	for _, feature := range features {
		feature, ok := normalizeFeature(feature)
		if !ok {
			continue
		}
		for _, lookup := range g.features[feature.Tag] {
			for _, st := range lookup.subtables {
				if st.single != nil {
					for i, gid := range out {
						if replacement, ok := st.single[gid]; ok {
							out[i] = replacement
						}
					}
				}
				if st.alternates != nil {
					altIndex := int(feature.Value - 1)
					for i, gid := range out {
						alts := st.alternates[gid]
						if altIndex >= 0 && altIndex < len(alts) {
							out[i] = alts[altIndex]
						}
					}
				}
			}
		}
	}
	return out
}

// normalizeFeature enforces OpenType's exact four-byte, case-sensitive tag
// syntax and drops disabled or unsupported features before traversal.
func normalizeFeature(feature Feature) (Feature, bool) {
	if feature.Value == 0 || len(feature.Tag) != 4 {
		return Feature{}, false
	}
	for i := 0; i < len(feature.Tag); i++ {
		if feature.Tag[i] < 0x20 || feature.Tag[i] > 0x7e {
			return Feature{}, false
		}
	}
	if _, ok := mathFeatureTags[feature.Tag]; !ok {
		return Feature{}, false
	}
	return feature, true
}

func subtableAllowHeader(b []byte, off int) ([]byte, error) {
	if off <= 0 || off >= len(b) {
		return nil, fmt.Errorf("offset %d outside %d-byte table", off, len(b))
	}
	return b[off:], nil
}
