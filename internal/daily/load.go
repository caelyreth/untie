package daily

import (
	"errors"
	"os"

	"github.com/caelyreth/untie"
)

// LoadEngine prepares the tracked NewCM font for corpus evaluation.
func LoadEngine(fontPath string) (*untie.Engine, error) {
	if fontPath == "" {
		return nil, errors.New("daily: empty font path")
	}
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, err
	}
	fonts, err := untie.LoadFonts(untie.Fonts{
		Math: untie.FontData{Bytes: fontBytes, ID: "NewCMMath-Book"},
	})
	if err != nil {
		return nil, err
	}
	return untie.NewEngine(fonts, untie.Limits{})
}
