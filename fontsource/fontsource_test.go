package fontsource_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/fontsource"
)

func TestReadFace(t *testing.T) {
	t.Parallel()

	data := testFontBytes(t)
	fsys := fstest.MapFS{"math.otf": {Data: data}}
	face, err := fontsource.ReadFace(fsys, "math.otf",
		font.WithFontID("custom"), font.WithFontIndex(0))
	if err != nil {
		t.Fatalf("ReadFace: %v", err)
	}
	if face.ID != "custom" || face.Index != 0 || string(face.Bytes) != string(data) {
		t.Fatalf("ReadFace = %#v", face)
	}
}

func TestLoadMath(t *testing.T) {
	t.Parallel()

	data := testFontBytes(t)
	fsys := fstest.MapFS{"math.otf": {Data: data}}
	set, err := fontsource.LoadMath(fsys, "math.otf")
	if err != nil {
		t.Fatalf("LoadMath: %v", err)
	}
	info, ok := set.Face(font.MathFace)
	if !ok || !strings.HasPrefix(info.ID, "math-newcmmath-book-") {
		t.Fatalf("math face = %#v, %v", info, ok)
	}
}

func TestLoadSet(t *testing.T) {
	t.Parallel()

	data := testFontBytes(t)
	fsys := fstest.MapFS{
		"math.otf":     {Data: data},
		"text.otf":     {Data: data},
		"fallback.otf": {Data: data},
	}
	text := &fontsource.Face{Path: "text.otf"}
	set, err := fontsource.LoadSet(fsys, fontsource.Config{
		Math:      fontsource.Face{Path: "math.otf"},
		Text:      text,
		Fallbacks: []fontsource.Face{{Path: "fallback.otf"}},
	})
	if err != nil {
		t.Fatalf("LoadSet: %v", err)
	}
	fallbackRef, _ := font.FallbackFace(0)
	mathInfo, _ := set.Face(font.MathFace)
	textInfo, _ := set.Face(font.TextFace)
	fallbackInfo, _ := set.Face(fallbackRef)
	if mathInfo.ID == textInfo.ID || mathInfo.ID == fallbackInfo.ID || textInfo.ID == fallbackInfo.ID {
		t.Fatalf("derived IDs are not unique: %q, %q, %q", mathInfo.ID, textInfo.ID, fallbackInfo.ID)
	}
}

func TestLoadSetPreservesVariationCoordinates(t *testing.T) {
	t.Parallel()

	mathData := testFontBytes(t)
	textData, err := os.ReadFile("../testdata/upstream/fonts/SourceSans-VF-MATH.ttf")
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{
		"math.otf": {Data: mathData},
		"text.ttf": {Data: textData},
	}
	set, err := fontsource.LoadSet(fsys, fontsource.Config{
		Math: fontsource.Face{Path: "math.otf"},
		Text: &fontsource.Face{
			Path:       "text.ttf",
			Variations: []font.Variation{{Tag: "wght", Value: 650}},
		},
	})
	if err != nil {
		t.Fatalf("LoadSet: %v", err)
	}
	coordinates, ok := set.VariationCoordinates(font.TextFace)
	if !ok || len(coordinates) != 1 || coordinates[0] != (font.Variation{Tag: "wght", Value: 650}) {
		t.Fatalf("text coordinates = %+v, %v", coordinates, ok)
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()

	data := testFontBytes(t)
	fsys := fstest.MapFS{
		"math.otf": {Data: data},
		"bad.otf":  {Data: []byte("not a font")},
		"folder":   {Mode: fs.ModeDir},
	}
	if _, err := fontsource.ReadFace(nil, "math.otf"); err == nil {
		t.Fatal("ReadFace(nil filesystem) error = nil")
	}
	if _, err := fontsource.ReadFace(fsys, "missing.otf"); err == nil {
		t.Fatal("ReadFace(missing) error = nil")
	}
	if _, err := fontsource.ReadFace(fsys, "folder"); err == nil {
		t.Fatal("ReadFace(directory) error = nil")
	}
	if _, err := fontsource.LoadMath(fsys, "bad.otf"); err == nil {
		t.Fatal("LoadMath(malformed) error = nil")
	}
	if _, err := fontsource.LoadSet(fsys, fontsource.Config{
		Math: fontsource.Face{Path: "math.otf"},
		Text: &fontsource.Face{Path: "missing.otf"},
	}); err == nil {
		t.Fatal("LoadSet(missing text) error = nil")
	}
}

func testFontBytes(t testing.TB) []byte {
	t.Helper()
	data, err := os.ReadFile("../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		t.Fatalf("read test font: %v", err)
	}
	return data
}
