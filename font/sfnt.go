package font

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"unicode/utf16"

	tsfont "github.com/go-text/typesetting/font"
)

const (
	maxFaceBytes   = 256 << 20
	maxSFNTTables  = 4096
	sfntHeaderLen  = 12
	tableRecordLen = 16
)

type parsedFace struct {
	data      FontFace
	info      FaceInfo
	glyphFont *tsfont.Font
	sfntBytes []byte
	tables    map[string][]byte
	math      mathTable
	gsub      gsubTable
	metrics   []GlyphMetrics
	vertical  VerticalMetrics
	outlineMu outlineCache
}

type tableRecord struct {
	tag    string
	offset uint32
	length uint32
}

// openFace selects index from a single SFNT or collection and returns a
// standalone SFNT plus validated table views. The returned bytes are always
// owned by the caller.
func openFace(data []byte, index int) ([]byte, map[string][]byte, error) {
	if len(data) == 0 {
		return nil, nil, errors.New("empty font data")
	}
	if len(data) > maxFaceBytes {
		return nil, nil, fmt.Errorf("font data is %d bytes; limit is %d", len(data), maxFaceBytes)
	}
	if index < 0 {
		return nil, nil, fmt.Errorf("negative face index %d", index)
	}

	faceOffset := uint32(0)
	if len(data) >= 4 && string(data[:4]) == "ttcf" {
		faceOffset, err := collectionFaceOffset(data, index)
		if err != nil {
			return nil, nil, err
		}
		standalone, records, err := extractCollectionFace(data, faceOffset)
		if err != nil {
			return nil, nil, err
		}
		tables, err := standaloneTables(standalone, records)
		return standalone, tables, err
	}
	if index != 0 {
		return nil, nil, fmt.Errorf("face index %d requested from a non-collection font", index)
	}

	records, err := sfntTableRecords(data, faceOffset)
	if err != nil {
		return nil, nil, err
	}
	tables, err := dataTables(data, records)
	if err != nil {
		return nil, nil, err
	}
	return data, tables, nil
}

func collectionFaceOffset(data []byte, index int) (uint32, error) {
	if len(data) < 12 {
		return 0, errors.New("truncated TTC header")
	}
	count := uint64(binary.BigEndian.Uint32(data[8:12]))
	if count == 0 {
		return 0, errors.New("TTC contains no faces")
	}
	if uint64(len(data)) < 12+4*count {
		return 0, errors.New("truncated TTC face offset array")
	}
	if index < 0 || uint64(index) >= count {
		return 0, fmt.Errorf("face index %d out of range for %d-face collection", index, count)
	}
	off := uint64(binary.BigEndian.Uint32(data[12+4*index : 16+4*index]))
	if off < 12+4*count || off+sfntHeaderLen > uint64(len(data)) {
		return 0, fmt.Errorf("invalid TTC face offset %d", off)
	}
	return uint32(off), nil
}

func sfntTableRecords(data []byte, faceOffset uint32) ([]tableRecord, error) {
	base := uint64(faceOffset)
	if base+sfntHeaderLen > uint64(len(data)) {
		return nil, errors.New("truncated SFNT offset table")
	}
	magic := binary.BigEndian.Uint32(data[base : base+4])
	if magic != 0x00010000 && magic != 0x4f54544f && string(data[base:base+4]) != "true" {
		return nil, fmt.Errorf("unsupported or invalid SFNT version 0x%08x", magic)
	}
	count := int(binary.BigEndian.Uint16(data[base+4 : base+6]))
	if count <= 0 || count > maxSFNTTables {
		return nil, fmt.Errorf("invalid SFNT table count %d", count)
	}
	end := base + sfntHeaderLen + uint64(count)*tableRecordLen
	if end > uint64(len(data)) {
		return nil, errors.New("truncated SFNT table directory")
	}

	records := make([]tableRecord, count)
	seen := make(map[string]struct{}, count)
	for i := range records {
		off := base + sfntHeaderLen + uint64(i)*tableRecordLen
		tag := string(data[off : off+4])
		if _, exists := seen[tag]; exists {
			return nil, fmt.Errorf("duplicate SFNT table %q", tag)
		}
		seen[tag] = struct{}{}
		tableOff := uint64(binary.BigEndian.Uint32(data[off+8 : off+12]))
		tableLen := uint64(binary.BigEndian.Uint32(data[off+12 : off+16]))
		if tableOff > uint64(len(data)) || tableLen > uint64(len(data))-tableOff {
			return nil, fmt.Errorf("SFNT table %q extends past end of data", tag)
		}
		records[i] = tableRecord{tag: tag, offset: uint32(tableOff), length: uint32(tableLen)}
	}
	return records, nil
}

func dataTables(data []byte, records []tableRecord) (map[string][]byte, error) {
	tables := make(map[string][]byte, len(records))
	for _, rec := range records {
		end := uint64(rec.offset) + uint64(rec.length)
		if end > uint64(len(data)) {
			return nil, fmt.Errorf("SFNT table %q is truncated", rec.tag)
		}
		tables[rec.tag] = data[rec.offset:end]
	}
	return tables, nil
}

func standaloneTables(data []byte, records []tableRecord) (map[string][]byte, error) {
	tables := make(map[string][]byte, len(records))
	for i, rec := range records {
		// extractCollectionFace writes records in input order and rewrites offsets
		// to the extracted file. Re-read them rather than trusting rec.offset.
		off := sfntHeaderLen + i*tableRecordLen
		tableOff := uint64(binary.BigEndian.Uint32(data[off+8 : off+12]))
		tableLen := uint64(binary.BigEndian.Uint32(data[off+12 : off+16]))
		if tableOff > uint64(len(data)) || tableLen > uint64(len(data))-tableOff {
			return nil, fmt.Errorf("extracted table %q is truncated", rec.tag)
		}
		tables[rec.tag] = data[tableOff : tableOff+tableLen]
	}
	return tables, nil
}

func extractCollectionFace(data []byte, faceOffset uint32) ([]byte, []tableRecord, error) {
	records, err := sfntTableRecords(data, faceOffset)
	if err != nil {
		return nil, nil, err
	}
	ordered := append([]tableRecord(nil), records...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].offset < ordered[j].offset
	})

	var payload uint64
	for _, rec := range ordered {
		payload += align4(uint64(rec.length))
	}
	total := uint64(sfntHeaderLen) + uint64(len(records))*tableRecordLen + payload
	if total > maxFaceBytes {
		return nil, nil, fmt.Errorf("extracted face would be %d bytes; limit is %d", total, maxFaceBytes)
	}

	out := make([]byte, int(total))
	copy(out[:4], data[faceOffset:faceOffset+4])
	writeSFNTSearchHeader(out[4:12], len(records))

	dataOffset := uint64(sfntHeaderLen) + uint64(len(records))*tableRecordLen
	newOffsets := make(map[int]uint32, len(records))
	byOriginal := make(map[int]uint32, len(ordered))
	for _, rec := range ordered {
		key := indexRecord(records, rec)
		byOriginal[key] = uint32(dataOffset)
		tableEnd := uint64(rec.offset) + uint64(rec.length)
		if tableEnd > uint64(len(data)) {
			return nil, nil, fmt.Errorf("SFNT table %q is truncated", rec.tag)
		}
		copy(out[dataOffset:dataOffset+uint64(rec.length)], data[rec.offset:tableEnd])
		dataOffset += align4(uint64(rec.length))
	}

	for i, rec := range records {
		off := sfntHeaderLen + i*tableRecordLen
		copy(out[off:off+4], rec.tag)
		// The checksum is computed over the original table bytes, which are copied
		// unchanged. Do not trust or rewrite it.
		copy(out[off+4:off+8], data[faceOffset+sfntHeaderLen+uint32(i)*tableRecordLen+4:faceOffset+sfntHeaderLen+uint32(i)*tableRecordLen+8])
		newOffsets[i] = byOriginal[indexRecord(records, rec)]
		binary.BigEndian.PutUint32(out[off+8:off+12], newOffsets[i])
		binary.BigEndian.PutUint32(out[off+12:off+16], rec.length)
	}
	return out, records, nil
}

func indexRecord(records []tableRecord, target tableRecord) int {
	for i := range records {
		if records[i] == target {
			return i
		}
	}
	return -1
}

func writeSFNTSearchHeader(dst []byte, count int) {
	pow2 := 1
	entrySelector := 0
	for pow2*2 <= count {
		pow2 *= 2
		entrySelector++
	}
	searchRange := pow2 * tableRecordLen
	binary.BigEndian.PutUint16(dst[0:2], uint16(count))
	binary.BigEndian.PutUint16(dst[2:4], uint16(searchRange))
	binary.BigEndian.PutUint16(dst[4:6], uint16(entrySelector))
	binary.BigEndian.PutUint16(dst[6:8], uint16(count*tableRecordLen-searchRange))
}

func align4(v uint64) uint64 {
	return (v + 3) &^ 3
}

func parseSelectedFace(data FontFace) (*parsedFace, error) {
	standalone, tables, err := openFace(data.Bytes, data.Index)
	if err != nil {
		return nil, fmt.Errorf("face %d (%q): %w", data.Index, data.ID, err)
	}
	tsFaces, err := tsfont.ParseTTC(bytes.NewReader(standalone))
	if err != nil {
		return nil, fmt.Errorf("face %d (%q): parse outline tables: %w", data.Index, data.ID, err)
	}
	if len(tsFaces) != 1 {
		return nil, fmt.Errorf("face %d (%q): extracted SFNT contains %d faces", data.Index, data.ID, len(tsFaces))
	}
	glyphFont := tsFaces[0].Font
	if glyphFont.Upem() == 0 {
		return nil, fmt.Errorf("face %d (%q): invalid units per em 0", data.Index, data.ID)
	}
	glyphCount, err := readGlyphCount(tables["maxp"])
	if err != nil {
		return nil, fmt.Errorf("face %d (%q): read glyph count: %w", data.Index, data.ID, err)
	}

	name, err := postScriptName(tables["name"])
	if err != nil {
		return nil, fmt.Errorf("face %d (%q): read PostScript name: %w", data.Index, data.ID, err)
	}
	if name == "" {
		return nil, fmt.Errorf("face %d (%q): empty PostScript name", data.Index, data.ID)
	}

	m := mathTable{}
	if mathBytes, ok := tables["MATH"]; ok {
		m, err = parseMathTable(mathBytes)
		if err != nil {
			return nil, fmt.Errorf("face %d (%q): parse MATH: %w", data.Index, data.ID, err)
		}
	}
	g, err := parseGSUBTable(tables["GSUB"])
	if err != nil {
		return nil, fmt.Errorf("face %d (%q): parse GSUB: %w", data.Index, data.ID, err)
	}
	if err := m.validateGlyphIDs(glyphCount); err != nil {
		return nil, fmt.Errorf("face %d (%q): validate MATH glyph references: %w", data.Index, data.ID, err)
	}
	if err := g.validateGlyphIDs(glyphCount); err != nil {
		return nil, fmt.Errorf("face %d (%q): validate GSUB glyph references: %w", data.Index, data.ID, err)
	}

	pf := &parsedFace{
		data: data,
		info: FaceInfo{
			ID:             data.ID,
			Index:          data.Index,
			PostScriptName: name,
			UnitsPerEm:     glyphFont.Upem(),
			GlyphCount:     glyphCount,
			HasMath:        m.present,
		},
		glyphFont: glyphFont,
		sfntBytes: standalone,
		tables:    tables,
		math:      m,
		gsub:      g,
	}
	if err := pf.readVerticalMetrics(); err != nil {
		return nil, err
	}
	if err := pf.readGlyphMetrics(); err != nil {
		return nil, err
	}
	return pf, nil
}

func (f *parsedFace) readVerticalMetrics() error {
	fontBounds, err := readFontBounds(f.tables["head"])
	if err != nil {
		return fmt.Errorf("face %d (%q): read font bounds: %w", f.data.Index, f.data.ID, err)
	}
	face := tsfont.NewFace(f.glyphFont)
	extents, ok := face.FontHExtents()
	if !ok {
		return fmt.Errorf("face %d (%q): missing horizontal font metrics", f.data.Index, f.data.ID)
	}
	ascender, err := roundedMetric(extents.Ascender, "ascender")
	if err != nil {
		return fmt.Errorf("face %d (%q): %w", f.data.Index, f.data.ID, err)
	}
	descender, err := roundedMetric(extents.Descender, "descender")
	if err != nil {
		return fmt.Errorf("face %d (%q): %w", f.data.Index, f.data.ID, err)
	}
	lineGap, err := roundedMetric(extents.LineGap, "line gap")
	if err != nil {
		return fmt.Errorf("face %d (%q): %w", f.data.Index, f.data.ID, err)
	}
	capHeight, err := roundedMetric(face.LineMetric(tsfont.CapHeight), "cap height")
	if err != nil {
		return fmt.Errorf("face %d (%q): %w", f.data.Index, f.data.ID, err)
	}
	xHeight, err := roundedMetric(face.LineMetric(tsfont.XHeight), "x-height")
	if err != nil {
		return fmt.Errorf("face %d (%q): %w", f.data.Index, f.data.ID, err)
	}
	f.vertical = VerticalMetrics{
		Ascender:   ascender,
		Descender:  descender,
		LineGap:    lineGap,
		CapHeight:  capHeight,
		XHeight:    xHeight,
		FontBounds: fontBounds,
	}
	return nil
}

func (f *parsedFace) mapRune(r rune) (GlyphID, bool) {
	gid, ok := f.glyphFont.NominalGlyph(r)
	if !ok {
		return 0, false
	}
	return GlyphID(gid), true
}

func postScriptName(b []byte) (string, error) {
	if len(b) < 6 {
		return "", errors.New("name table is truncated")
	}
	format := binary.BigEndian.Uint16(b[0:2])
	if format > 1 {
		return "", fmt.Errorf("unsupported name table format %d", format)
	}
	count := int(binary.BigEndian.Uint16(b[2:4]))
	stringOffset := int(binary.BigEndian.Uint16(b[4:6]))
	if count > (len(b)-6)/12 {
		return "", errors.New("name records are truncated")
	}
	best := ""
	bestRank := -1
	for i := 0; i < count; i++ {
		off := 6 + i*12
		platform := binary.BigEndian.Uint16(b[off : off+2])
		encoding := binary.BigEndian.Uint16(b[off+2 : off+4])
		language := binary.BigEndian.Uint16(b[off+4 : off+6])
		nameID := binary.BigEndian.Uint16(b[off+6 : off+8])
		if nameID != 6 {
			continue
		}
		length := int(binary.BigEndian.Uint16(b[off+8 : off+10]))
		start := stringOffset + int(binary.BigEndian.Uint16(b[off+10:off+12]))
		if start < 0 || length < 0 || start > len(b) || length > len(b)-start {
			return "", errors.New("name string extends past table")
		}
		raw := b[start : start+length]
		var name string
		rank := 0
		switch {
		case platform == 3 && (encoding == 1 || encoding == 10):
			decoded, err := decodeUTF16BE(raw)
			if err != nil {
				return "", err
			}
			name = decoded
			rank = 30
			if language == 0x0409 {
				rank = 40
			}
		case platform == 0:
			decoded, err := decodeUTF16BE(raw)
			if err != nil {
				return "", err
			}
			name = decoded
			rank = 20
		case platform == 1 && encoding == 0:
			name = string(raw)
			rank = 10
		}
		if name != "" && rank > bestRank {
			best, bestRank = name, rank
		}
	}
	if best == "" {
		return "", errors.New("name ID 6 not found")
	}
	return best, nil
}

func decodeUTF16BE(b []byte) (string, error) {
	if len(b)%2 != 0 {
		return "", errors.New("odd-length UTF-16 name")
	}
	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = binary.BigEndian.Uint16(b[2*i : 2*i+2])
	}
	return string(utf16.Decode(units)), nil
}
