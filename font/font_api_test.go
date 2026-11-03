package font

import (
	"strings"
	"testing"
)

func TestLoadMathFontDerivesStableSafeID(t *testing.T) {
	t.Parallel()

	data := readTestFont(t)
	original := append([]byte(nil), data...)
	first, err := LoadMathFont(data)
	if err != nil {
		t.Fatalf("LoadMathFont: %v", err)
	}
	second, err := LoadMathFont(append([]byte(nil), data...))
	if err != nil {
		t.Fatalf("LoadMathFont second: %v", err)
	}
	info, ok := first.Face(MathFace)
	if !ok {
		t.Fatal("Face(MathFace) = false")
	}
	other, _ := second.Face(MathFace)
	if info.ID == "" || info.ID != other.ID {
		t.Fatalf("derived IDs = %q and %q", info.ID, other.ID)
	}
	if !strings.HasPrefix(info.ID, "math-newcmmath-book-") {
		t.Fatalf("derived ID = %q, want role and PostScript metadata", info.ID)
	}
	if got := first.Fonts().Math.ID; got != info.ID {
		t.Fatalf("Fonts().Math.ID = %q, want %q", got, info.ID)
	}
	for _, r := range info.ID {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			t.Fatalf("derived ID %q contains unsafe rune %q", info.ID, r)
		}
	}
	if first.KeySHA256() != second.KeySHA256() {
		t.Fatal("stable derived IDs produced different font keys")
	}
	data[0] ^= 0xff
	if got := first.Fonts().Math.Bytes; string(got) != string(original) {
		t.Fatal("LoadMathFont retained caller-owned font bytes")
	}
	if _, ok := first.MapRune(MathFace, 'A'); !ok {
		t.Fatal("caller mutation affected the loaded face")
	}
}

func TestLoadMathFontOptions(t *testing.T) {
	t.Parallel()

	data := makeTTC(t, readTestFont(t), 2)
	set, err := LoadMathFont(data, WithFontID("host-math"), WithFontIndex(1))
	if err != nil {
		t.Fatalf("LoadMathFont: %v", err)
	}
	info, ok := set.Face(MathFace)
	if !ok {
		t.Fatal("Face(MathFace) = false")
	}
	if info.ID != "host-math" || info.Index != 1 {
		t.Fatalf("face info = %#v", info)
	}
	indexed, err := LoadMathFont(data, WithFontIndex(1))
	if err != nil {
		t.Fatalf("LoadMathFont derived TTC face: %v", err)
	}
	indexedInfo, _ := indexed.Face(MathFace)
	first, err := LoadMathFont(data)
	if err != nil {
		t.Fatalf("LoadMathFont derived TTC face 0: %v", err)
	}
	firstInfo, _ := first.Face(MathFace)
	if indexedInfo.ID == firstInfo.ID {
		t.Fatal("TTC index did not distinguish derived face IDs")
	}
}

func TestLoadFontSetDerivesUniqueRoleIDs(t *testing.T) {
	t.Parallel()

	data := readTestFont(t)
	text := FontFace{Bytes: data}
	set, err := LoadFontSet(FontSetConfig{
		Math:      FontFace{Bytes: data},
		Text:      &text,
		Fallbacks: []FontFace{{Bytes: data}},
	})
	if err != nil {
		t.Fatalf("LoadFontSet: %v", err)
	}
	mathInfo, _ := set.Face(MathFace)
	textInfo, _ := set.Face(TextFace)
	fallbackRef, _ := FallbackFace(0)
	fallbackInfo, _ := set.Face(fallbackRef)
	ids := []string{mathInfo.ID, textInfo.ID, fallbackInfo.ID}
	seen := make(map[string]bool, len(ids))
	for i, id := range ids {
		if id == "" || seen[id] {
			t.Fatalf("role IDs = %v", ids)
		}
		seen[id] = true
		wantPrefix := []string{"math-", "text-", "fallback-0-"}[i]
		if !strings.HasPrefix(id, wantPrefix) {
			t.Fatalf("role ID %q does not have prefix %q", id, wantPrefix)
		}
	}
	if got := set.Fonts(); got.Math.ID != mathInfo.ID || got.Text.ID != textInfo.ID || got.Fallbacks[0].ID != fallbackInfo.ID {
		t.Fatalf("Fonts() IDs = %#v", got)
	}
}

func TestLoadFontSetRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()

	data := readTestFont(t)
	text := FontFace{Bytes: data, ID: "same"}
	if _, err := LoadFontSet(FontSetConfig{
		Math: FontFace{Bytes: data, ID: "same"},
		Text: &text,
	}); err == nil {
		t.Fatal("LoadFontSet accepted duplicate explicit IDs")
	}

	derivedText, err := LoadFontSet(FontSetConfig{
		Math: FontFace{Bytes: data, ID: "math"},
		Text: &FontFace{Bytes: data},
	})
	if err != nil {
		t.Fatalf("derive text ID: %v", err)
	}
	textInfo, _ := derivedText.Face(TextFace)
	if _, err := LoadFontSet(FontSetConfig{
		Math: FontFace{Bytes: data, ID: textInfo.ID},
		Text: &FontFace{Bytes: data},
	}); err == nil {
		t.Fatal("LoadFontSet accepted an explicit ID colliding with a derived ID")
	}
}

func TestLoadFontSetRejectsMalformedData(t *testing.T) {
	t.Parallel()

	if _, err := LoadMathFont([]byte("not a font")); err == nil {
		t.Fatal("LoadMathFont accepted malformed data")
	}
	if _, err := LoadFontSet(FontSetConfig{}); err == nil {
		t.Fatal("LoadFontSet accepted a missing math face")
	}
	data := makeTTC(t, readTestFont(t), 2)
	if _, err := LoadMathFont(data, WithFontIndex(2)); err == nil {
		t.Fatal("LoadMathFont accepted an out-of-range TTC index")
	}
}

func TestSafeIDSegment(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"NewCM Math Book": "newcm-math-book",
		"  A/B__C  ":      "a-b-c",
		"é":               "",
		"123":             "123",
	} {
		if got := safeIDSegment(input); got != want {
			t.Errorf("safeIDSegment(%q) = %q, want %q", input, got, want)
		}
	}
}
