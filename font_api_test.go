package untie

import (
	"errors"
	"strings"
	"testing"
)

func TestFontSetupAliasesAndFacade(t *testing.T) {
	t.Parallel()

	data := testFontBytes(t)
	config := FontSetConfig{
		Math: FontFace{Bytes: data},
	}
	set, err := LoadFontSet(config)
	if err != nil {
		t.Fatalf("LoadFontSet: %v", err)
	}
	info, ok := set.Face(0)
	if !ok || !strings.HasPrefix(info.ID, "math-") {
		t.Fatalf("Face(0) = %#v, %v", info, ok)
	}

	set, err = LoadMathFont(data, WithFontID("host-math"), WithFontIndex(0))
	if err != nil {
		t.Fatalf("LoadMathFont: %v", err)
	}
	info, _ = set.Face(0)
	if info.ID != "host-math" {
		t.Fatalf("WithFontID produced %q", info.ID)
	}

	if _, err := LoadMathFont([]byte("not a font")); !errors.Is(err, ErrInvalidFonts) {
		t.Fatalf("LoadMathFont malformed error = %v, want ErrInvalidFonts", err)
	}
}
