package daily

import (
	"errors"
	"os"

	"github.com/caelyreth/untie"
)

// LoadFontSet prepares the tracked NewCM font set for per-case Engine creation.
func LoadFontSet(fontPath string) (*untie.FontSet, error) {
	if fontPath == "" {
		return nil, errors.New("daily: empty font path")
	}
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, err
	}
	return untie.LoadFontSet(untie.FontSetConfig{
		Math: untie.FontFace{Bytes: fontBytes, ID: "NewCMMath-Book"},
	})
}
