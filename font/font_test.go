package font

import (
	"bytes"
	"os"
	"runtime"
	"sync"
	"testing"
)

func TestLoadNewCM(t *testing.T) {
	t.Parallel()
	set := loadSharedTestFont(t)
	info, ok := set.Face(MathFace)
	if !ok {
		t.Fatal("Face(MathFace) = false")
	}
	if info.PostScriptName != "NewCMMath-Book" {
		t.Fatalf("PostScriptName = %q", info.PostScriptName)
	}
	if info.UnitsPerEm != 1000 {
		t.Fatalf("UnitsPerEm = %d", info.UnitsPerEm)
	}
	if info.GlyphCount != 8598 {
		t.Fatalf("GlyphCount = %d", info.GlyphCount)
	}
	if !info.HasMath {
		t.Fatal("HasMath = false")
	}
	if info.ID != "newcm-math-book" || info.Index != 0 {
		t.Fatalf("face identity = %#v", info)
	}
	if set.FaceCount() != 1 {
		t.Fatalf("FaceCount = %d", set.FaceCount())
	}
	if set.Key() == 0 && set.KeySHA256() == ([32]byte{}) {
		t.Fatal("font key is empty")
	}
}

func TestInputAndOutputOwnership(t *testing.T) {
	t.Parallel()
	data := readTestFont(t)
	original := append([]byte(nil), data...)
	set, err := Load(Fonts{Math: FontData{Bytes: data, ID: "owned"}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	key := set.KeySHA256()
	data[0] ^= 0xff
	got, ok := set.MapRune(MathFace, 'A')
	if !ok || got.ID == 0 {
		t.Fatalf("MapRune after caller mutation = %#v, %v", got, ok)
	}
	if set.KeySHA256() != key {
		t.Fatal("caller mutation changed key")
	}
	copied := set.Fonts()
	if !bytes.Equal(copied.Math.Bytes, original) {
		t.Fatal("Fonts() did not return the original bytes")
	}
	copied.Math.Bytes[0] ^= 0xff
	if set.Fonts().Math.Bytes[0] != original[0] {
		t.Fatal("Fonts() returned shared storage")
	}
}

func TestDeterministicKey(t *testing.T) {
	t.Parallel()
	data := readTestFont(t)
	a, err := Load(Fonts{Math: FontData{Bytes: data, ID: "same"}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load(Fonts{Math: FontData{Bytes: append([]byte(nil), data...), ID: "same"}})
	if err != nil {
		t.Fatal(err)
	}
	if a.Key() != b.Key() || a.KeySHA256() != b.KeySHA256() {
		t.Fatal("identical font sets have different keys")
	}
	c, err := Load(Fonts{Math: FontData{Bytes: data, ID: "different"}})
	if err != nil {
		t.Fatal(err)
	}
	if a.KeySHA256() == c.KeySHA256() {
		t.Fatal("font ID does not participate in the key")
	}
	var nilSet *FontSet
	if nilSet.Key() != 0 || nilSet.KeySHA256() != ([32]byte{}) {
		t.Fatal("nil FontSet returned a nonzero key")
	}
}

func TestTTCFaceSelection(t *testing.T) {
	t.Parallel()
	data := makeTTC(t, readTestFont(t), 2)
	set, err := Load(Fonts{Math: FontData{Bytes: data, Index: 1, ID: "ttc-face-1"}})
	if err != nil {
		t.Fatalf("Load TTC face 1: %v", err)
	}
	info, _ := set.Face(MathFace)
	if info.Index != 1 || info.PostScriptName != "NewCMMath-Book" {
		t.Fatalf("selected face = %#v", info)
	}
	if _, err := Load(Fonts{Math: FontData{Bytes: data, Index: 2, ID: "bad"}}); err == nil {
		t.Fatal("Load accepted an out-of-range TTC index")
	}
	if _, err := Load(Fonts{Math: FontData{Bytes: readTestFont(t), Index: 1, ID: "bad"}}); err == nil {
		t.Fatal("Load accepted a nonzero index for a single SFNT")
	}
}

func TestTextAndFallbacks(t *testing.T) {
	t.Parallel()
	data := readTestFont(t)
	text := FontData{Bytes: data, ID: "text"}
	set, err := Load(Fonts{
		Math:      FontData{Bytes: data, ID: "math"},
		Text:      &text,
		Fallbacks: []FontData{{Bytes: data, ID: "fallback"}},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ref, ok := FallbackFace(0)
	if !ok {
		t.Fatal("FallbackFace(0) = false")
	}
	for _, tc := range []struct {
		ref  FaceRef
		id   string
		want bool
	}{
		{MathFace, "math", true},
		{TextFace, "text", true},
		{ref, "fallback", true},
		{ref + 1, "", false},
	} {
		info, ok := set.Face(tc.ref)
		if ok != tc.want {
			t.Fatalf("Face(%d) ok = %v, want %v", tc.ref, ok, tc.want)
		}
		if ok && info.ID != tc.id {
			t.Fatalf("Face(%d).ID = %q, want %q", tc.ref, info.ID, tc.id)
		}
	}
	noText, err := Load(Fonts{Math: FontData{Bytes: data, ID: "math"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := noText.Face(TextFace); ok {
		t.Fatal("absent text face reported present")
	}
}

func TestFaceIDsMustBeUniqueAndNonEmpty(t *testing.T) {
	t.Parallel()
	data := readTestFont(t)
	tests := []struct {
		name  string
		fonts Fonts
	}{
		{
			name:  "empty math",
			fonts: Fonts{Math: FontData{Bytes: data}},
		},
		{
			name:  "blank math",
			fonts: Fonts{Math: FontData{Bytes: data, ID: " \t"}},
		},
		{
			name: "duplicate text",
			fonts: func() Fonts {
				text := FontData{Bytes: data, ID: "same"}
				return Fonts{Math: FontData{Bytes: data, ID: "same"}, Text: &text}
			}(),
		},
		{
			name: "duplicate fallback",
			fonts: Fonts{
				Math:      FontData{Bytes: data, ID: "same"},
				Fallbacks: []FontData{{Bytes: data, ID: "same"}},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Load(tc.fonts); err == nil {
				t.Fatal("Load() error = nil")
			}
		})
	}
}

func TestMalformedInputFailsAtLoad(t *testing.T) {
	t.Parallel()
	data := readTestFont(t)
	tests := []struct {
		name  string
		index int
		data  []byte
	}{
		{"empty", 0, nil},
		{"random", 0, []byte("not a font")},
		{"header only", 0, data[:11]},
		{"truncated directory", 0, data[:40]},
		{"truncated tables", 0, data[:len(data)/2]},
		{"bad index", -1, data},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(Fonts{Math: FontData{Bytes: tc.data, Index: tc.index}}); err == nil {
				t.Fatal("Load() error = nil")
			}
		})
	}
}

func TestConcurrentReads(t *testing.T) {
	set := loadSharedTestFont(t)
	glyph := Glyph{Face: MathFace, ID: 3548}
	var wg sync.WaitGroup
	workers := runtime.GOMAXPROCS(0) * 2
	errs := make(chan string, workers)
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				if _, ok := set.MapRune(MathFace, '\U0001D465'); !ok {
					errs <- "MapRune failed"
					return
				}
				if _, ok := set.GlyphMetrics(glyph); !ok {
					errs <- "GlyphMetrics failed"
					return
				}
				if _, ok := set.Outline(glyph); !ok {
					errs <- "Outline failed"
					return
				}
				if _, ok := set.MathConstruction(Glyph{Face: MathFace, ID: 9}, VerticalAxis); !ok {
					errs <- "MathConstruction failed"
					return
				}
				if _, ok := set.ShapeMath(MathFace, []GlyphID{34}, ScriptVariant, false); !ok {
					errs <- "ShapeMath failed"
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func BenchmarkOutlineCache(b *testing.B) {
	data, err := os.ReadFile(testFontPath)
	if err != nil {
		b.Fatal(err)
	}
	set, err := Load(Fonts{Math: FontData{Bytes: data, ID: "bench"}})
	if err != nil {
		b.Fatal(err)
	}
	g := Glyph{Face: MathFace, ID: 3548}
	if _, ok := set.Outline(g); !ok {
		b.Fatal("warm outline failed")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := set.Outline(g); !ok {
			b.Fatal("cached outline failed")
		}
	}
}

func FuzzLoad(f *testing.F) {
	data, err := os.ReadFile(testFontPath)
	if err == nil {
		if len(data) > 4096 {
			data = data[:4096]
		}
		f.Add(data)
	}
	f.Add([]byte("ttcf\x00\x01\x00\x00\x00\x00\x00\x01\x00\x00\x00\x0c"))
	f.Add([]byte("not a font"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Load(Fonts{Math: FontData{Bytes: data, ID: "fuzz"}})
	})
}
