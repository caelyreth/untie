// Package fontsource loads caller-supplied font files from an io/fs filesystem.
//
// It performs no system font discovery, environment lookup, or network access.
// Use it with os.DirFS, embed.FS, testing/fstest, or another host-owned fs.FS.
package fontsource

import (
	"fmt"
	"io/fs"

	"github.com/caelyreth/untie/font"
)

// Face identifies one font file and, optionally, a face inside a TTC.
//
// ID is optional. The font loader derives a stable ID when it is blank.
type Face struct {
	Path  string
	ID    string
	Index int
}

// Config identifies the files that make up a font set.
type Config struct {
	Math      Face
	Text      *Face
	Fallbacks []Face
}

// ReadFace reads one face file without parsing or copying its bytes beyond the
// filesystem read. LoadMath or LoadSet performs validation and ownership copies.
func ReadFace(fsys fs.FS, path string, opts ...font.FontOption) (font.FontFace, error) {
	if fsys == nil {
		return font.FontFace{}, fmt.Errorf("fontsource: nil filesystem")
	}
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return font.FontFace{}, fmt.Errorf("fontsource: read %q: %w", path, err)
	}
	face := font.FontFace{Bytes: data}
	for _, opt := range opts {
		if opt != nil {
			opt(&face)
		}
	}
	return face, nil
}

// LoadMath loads one math face from fsys.
func LoadMath(fsys fs.FS, path string, opts ...font.FontOption) (*font.FontSet, error) {
	face, err := ReadFace(fsys, path, opts...)
	if err != nil {
		return nil, err
	}
	return font.LoadFontSet(font.FontSetConfig{Math: face})
}

// LoadSet loads a complete math, text, and fallback configuration.
func LoadSet(fsys fs.FS, config Config) (*font.FontSet, error) {
	mathFace, err := config.Math.read(fsys)
	if err != nil {
		return nil, fmt.Errorf("math: %w", err)
	}
	loaded := font.FontSetConfig{Math: mathFace}
	if config.Text != nil {
		text, err := config.Text.read(fsys)
		if err != nil {
			return nil, fmt.Errorf("text: %w", err)
		}
		loaded.Text = &text
	}
	loaded.Fallbacks = make([]font.FontFace, len(config.Fallbacks))
	for i, source := range config.Fallbacks {
		face, err := source.read(fsys)
		if err != nil {
			return nil, fmt.Errorf("fallback %d: %w", i, err)
		}
		loaded.Fallbacks[i] = face
	}
	return font.LoadFontSet(loaded)
}

func (face Face) read(fsys fs.FS) (font.FontFace, error) {
	opts := []font.FontOption{font.WithFontIndex(face.Index)}
	if face.ID != "" {
		opts = append(opts, font.WithFontID(face.ID))
	}
	return ReadFace(fsys, face.Path, opts...)
}
