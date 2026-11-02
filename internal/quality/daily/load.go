package daily

import (
	"errors"
	"os"

	"github.com/caelyreth/untie"
)

// LoadFonts prepares the tracked NewCM font set for per-case Engine creation.
func LoadFonts(fontPath string) (*untie.FontSet, error) {
	if fontPath == "" {
		return nil, errors.New("daily: empty font path")
	}
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, err
	}
	return untie.LoadFonts(untie.Fonts{
		Math: untie.FontData{Bytes: fontBytes, ID: "NewCMMath-Book"},
	})
}
