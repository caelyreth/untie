package font

import "testing"

func FuzzParseGSUB(f *testing.F) {
	set := loadSharedTestFont(f)
	raw := set.face(MathFace).tables["GSUB"]
	f.Add(raw)
	for _, n := range []int{0, 1, 9, 10, 32, len(raw) / 2} {
		if n <= len(raw) {
			f.Add(raw[:n])
		}
	}
	f.Add([]byte("not a GSUB table"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseGSUBTable(data)
	})
}

func FuzzGSUBShape(f *testing.F) {
	f.Add(uint16(10), byte(1), "dtls")
	f.Add(uint16(65535), byte(0), "ssty")
	f.Add(uint16(0), byte(2), "")
	set := shapeTestFont()
	f.Fuzz(func(t *testing.T, rawGlyph uint16, rawValue byte, tag string) {
		out, ok := set.Shape(MathFace, []GlyphID{GlyphID(rawGlyph)}, Feature{
			Tag:   tag,
			Value: uint16(rawValue),
		})
		if !ok || len(out) != 1 {
			t.Fatalf("Shape = %v, %v", out, ok)
		}
		if out[0] == GlyphID(0) && rawGlyph != 0 {
			t.Fatalf("Shape produced glyph zero from %d", rawGlyph)
		}
	})
}
