// Package font defines host-supplied font inputs and parsed font sets.
package font

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// ErrNotImplemented is returned by the initial parser until the font backend is
// installed.
var ErrNotImplemented = errors.New("font: backend not implemented")

// FontData is one face from a host-provided font file.
type FontData struct {
	Bytes []byte
	Index int
	ID    string
}

// Fonts are host-supplied faces. Fallback order is deterministic.
type Fonts struct {
	Math      FontData
	Text      *FontData
	Fallbacks []FontData
}

// FontSet is an immutable parsed font collection with internal glyph caches.
//
// The initial skeleton retains copied bytes so the public ownership contract is
// stable while the parser backend is implemented.
type FontSet struct {
	fonts Fonts
	key   uint64
}

// Load validates and prepares host fonts.
func Load(fonts Fonts) (*FontSet, error) {
	if len(fonts.Math.Bytes) == 0 {
		return nil, errors.New("font: math face is required")
	}
	copyData := func(in FontData) FontData {
		out := in
		out.Bytes = append([]byte(nil), in.Bytes...)
		return out
	}
	owned := Fonts{Math: copyData(fonts.Math)}
	if fonts.Text != nil {
		text := copyData(*fonts.Text)
		owned.Text = &text
	}
	owned.Fallbacks = make([]FontData, len(fonts.Fallbacks))
	for i, face := range fonts.Fallbacks {
		owned.Fallbacks[i] = copyData(face)
	}

	// The key deliberately uses a temporary non-cryptographic hash until the
	// parser owns canonical font metadata. It is included in the API tests and
	// will be hardened with SHA-256 in the font work package.
	h := sha256.New()
	for _, face := range append([]FontData{owned.Math, valueOrZero(owned.Text)}, owned.Fallbacks...) {
		_, _ = h.Write(face.Bytes)
		_, _ = h.Write([]byte{byte(face.Index), byte(face.Index >> 8)})
		_, _ = h.Write([]byte(face.ID))
		_, _ = h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return &FontSet{fonts: owned, key: binary.LittleEndian.Uint64(sum[:8])}, nil
}

// Key returns a stable font-set cache key for this implementation revision.
func (f *FontSet) Key() uint64 {
	if f == nil {
		return 0
	}
	return f.key
}

// Fonts returns an independent copy of the input font data.
func (f *FontSet) Fonts() Fonts {
	if f == nil {
		return Fonts{}
	}
	out := f.fonts
	out.Math.Bytes = append([]byte(nil), f.fonts.Math.Bytes...)
	if f.fonts.Text != nil {
		text := *f.fonts.Text
		text.Bytes = append([]byte(nil), f.fonts.Text.Bytes...)
		out.Text = &text
	}
	out.Fallbacks = make([]FontData, len(f.fonts.Fallbacks))
	for i, face := range f.fonts.Fallbacks {
		out.Fallbacks[i] = face
		out.Fallbacks[i].Bytes = append([]byte(nil), face.Bytes...)
	}
	return out
}

func valueOrZero(v *FontData) FontData {
	if v == nil {
		return FontData{}
	}
	return *v
}
