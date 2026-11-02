package fixture

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestImportedFixtureCountsAndDecoders(t *testing.T) {
	t.Parallel()
	root := upstreamRoot(t)

	formulas, err := LoadFormulas(filepath.Join(root, "typlet-0.2.0", "corpus", "paired.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(formulas) != 81 {
		t.Fatalf("paired formulas = %d, want 81", len(formulas))
	}
	fixtures, err := LoadFixtures(filepath.Join(root, "typlet-0.2.0", "fixtures", "typst-0.15.1", "paired.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 81 {
		t.Fatalf("paired fixtures = %d, want 81", len(fixtures))
	}
	for i := range formulas {
		if formulas[i].ID != fixtures[i].ID {
			t.Fatalf("record %d: formula %q, fixture %q", i, formulas[i].ID, fixtures[i].ID)
		}
	}

	font, err := LoadFontDump(filepath.Join(root, "typlet-0.2.0", "fixtures", "typst-0.15.1", "font.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if font.Header.PostScriptName != "NewCMMath-Book" || font.Header.UnitsPerEm != 1000 {
		t.Fatalf("unexpected font header: %#v", font.Header)
	}
	if len(font.Glyphs) != 8598 || font.Header.GlyphCount != 8598 {
		t.Fatalf("glyphs = %d / %d, want 8598", len(font.Glyphs), font.Header.GlyphCount)
	}
	if len(font.CMap.Chars) != 4884 || len(font.CMap.Sequences) != 78 {
		t.Fatalf("cmap = %d chars, %d sequences", len(font.CMap.Chars), len(font.CMap.Sequences))
	}
	if len(font.Shapes) != 6 {
		t.Fatalf("shape records = %d, want 6", len(font.Shapes))
	}
}

func TestFrameFixtureRoundTrip(t *testing.T) {
	t.Parallel()
	root := upstreamRoot(t)
	fixtures, err := LoadFixtures(filepath.Join(root, "typlet-0.2.0", "fixtures", "typst-0.15.1", "paired.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range fixtures {
		if record.Frame == nil {
			continue
		}
		data, err := json.Marshal(record.Frame)
		if err != nil {
			t.Fatalf("%s: marshal: %v", record.ID, err)
		}
		var roundTrip Frame
		if err := json.Unmarshal(data, &roundTrip); err != nil {
			t.Fatalf("%s: unmarshal: %v\n%s", record.ID, err, data)
		}
		if diff := CompareFrames(record.ID, roundTrip, *record.Frame, 0); diff != nil {
			t.Fatalf("%s: round trip changed frame: %s", record.ID, diff)
		}
		converted, err := FrameToDIP(*record.Frame)
		if err != nil {
			t.Fatalf("%s: DIP conversion: %v", record.ID, err)
		}
		wantWidth := *record.Frame.Box.Width * DIPPerPoint
		if math.Abs(*converted.Box.Width-wantWidth) > 1e-12 {
			t.Fatalf("%s: DIP width = %v, want %v", record.ID, *converted.Box.Width, wantWidth)
		}
	}
}

func TestManifestAndAllCorporaLoad(t *testing.T) {
	t.Parallel()
	root := upstreamRoot(t)
	manifest, err := LoadCorpusManifest(filepath.Join(root, "manifests", "corpora.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Corpora) != 7 {
		t.Fatalf("corpora = %d, want 7", len(manifest.Corpora))
	}
	corpora, err := LoadAllCorpora(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, corpus := range corpora {
		total += len(corpus.Cases)
	}
	if total != 1282 {
		t.Fatalf("total cases = %d, want 1282", total)
	}
	selection, err := LoadTypletSelection(filepath.Join(root, "manifests", "typlet-selection.json"))
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]int{"features": 29, "code": 16, "symbols": 30, "syntax": 15} {
		ids, ok := selection.IDs(key)
		if !ok || len(ids) != want {
			t.Fatalf("selection %s = %d IDs (present %v), want %d", key, len(ids), ok, want)
		}
	}
}

func TestExactNewCMFontIsTracked(t *testing.T) {
	t.Parallel()
	root := upstreamRoot(t)
	fontPath := filepath.Join(root, "fonts", "NewCMMath-Book.otf")
	info, err := os.Stat(fontPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("font is empty")
	}
	hash, err := SHA256File(fontPath)
	if err != nil {
		t.Fatal(err)
	}
	const want = "2ea09ebc9167b1e1a66f31390dc917f2d4004ecfca72d51b28010e4ad6becd95"
	if hash != want {
		t.Fatalf("font hash = %s, want %s", hash, want)
	}
}

func TestFixturesWorkOutsideRepositoryCWD(t *testing.T) {
	root := upstreamRoot(t)
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
	manifest, err := LoadCorpusManifest(filepath.Join(root, "manifests", "corpora.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAllCorpora(root, manifest); err != nil {
		t.Fatal(err)
	}
}

func upstreamRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "upstream"))
}
