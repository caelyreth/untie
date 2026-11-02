package font

import "testing"

func TestLoadCopiesInput(t *testing.T) {
	t.Parallel()

	math := []byte{1, 2, 3}
	set, err := Load(Fonts{Math: FontData{Bytes: math, ID: "math"}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	math[0] = 9
	got := set.Fonts()
	if got.Math.Bytes[0] != 1 {
		t.Fatalf("Load retained caller storage: %v", got.Math.Bytes)
	}
	got.Math.Bytes[1] = 9
	if set.Fonts().Math.Bytes[1] != 2 {
		t.Fatal("Fonts() returned shared storage")
	}
	if set.Key() == 0 {
		t.Fatal("Key() = 0")
	}
}

func TestLoadRequiresMath(t *testing.T) {
	t.Parallel()

	if _, err := Load(Fonts{}); err == nil {
		t.Fatal("Load() error = nil, want failure")
	}
}
