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

func parseGSUBTable(b []byte) (gsubTable, error) {
	g := gsubTable{}
	if len(b) == 0 {
		return g, nil
	}
	if len(b) < 10 {
		return g, errors.New("GSUB header is truncated")
	}
	major := binary.BigEndian.Uint16(b[0:2])
	if major != 1 {
		return g, fmt.Errorf("unsupported GSUB major version %d", major)
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
	lookupOffsets, err := parseLookupOffsets(lookupBytes)
	if err != nil {
		return g, err
	}
	needed := make(map[int]struct{})
	for _, rec := range records {
		if _, ok := mathFeatureTags[rec.tag]; !ok {
			continue
		}
		for _, idx := range rec.lookups {
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
		for _, idx := range rec.lookups {
			g.features[rec.tag] = append(g.features[rec.tag], lookups[int(idx)])
		}
	}
	return g, nil
}

type featureRecord struct {
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

func (g gsubTable) shape(glyphs []GlyphID, features []Feature) []GlyphID {
	if len(glyphs) == 0 || len(features) == 0 || len(g.features) == 0 {
		return append([]GlyphID(nil), glyphs...)
	}
	out := append([]GlyphID(nil), glyphs...)
	for _, feature := range features {
		for _, lookup := range g.features[feature.Tag] {
			for _, st := range lookup.subtables {
				if st.single != nil && feature.Value != 0 {
					for i, gid := range out {
						if replacement, ok := st.single[gid]; ok {
							out[i] = replacement
						}
					}
				}
				if st.alternates != nil && feature.Value != 0 {
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

func subtableAllowHeader(b []byte, off int) ([]byte, error) {
	if off <= 0 || off >= len(b) {
		return nil, fmt.Errorf("offset %d outside %d-byte table", off, len(b))
	}
	return b[off:], nil
}
