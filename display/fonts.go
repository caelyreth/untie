package display

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/untie/source"
)

// FaceUse identifies the resolved face used for one or more shaped text
// clusters. Source ranges are in logical source order after layout
// finalization. Fallback is true when the face was not the first face requested
// for that text role.
type FaceUse struct {
	Source   source.Range
	FaceID   string
	Fallback bool
	Missing  bool
}

// MissingFallback identifies text for which none of TriedFaces supplied a
// usable outline cluster. It is structured lookup data, not a second
// diagnostic: recovery and CodeMissingGlyph diagnostics remain authoritative.
type MissingFallback struct {
	Source     source.Range
	Text       string
	TriedFaces []string
}

// FontUsage reports shaped text face selection for hosts that need to load
// fallback fonts ahead of future layouts or explain the current selection.
//
// Faces contains resolved face spans in source order. Missing contains merged,
// deduplicated fallback requirements. Math glyphs selected directly through
// OpenType MATH tables are not text clusters and are intentionally absent.
type FontUsage struct {
	Faces   []FaceUse
	Missing []MissingFallback
}

// UsesFallback reports whether any shaped text cluster used a non-primary face.
func (u FontUsage) UsesFallback() bool {
	for _, face := range u.Faces {
		if face.Fallback {
			return true
		}
	}
	return false
}

// NeedsFallback reports whether any shaped text cluster lacked coverage.
func (u FontUsage) NeedsFallback() bool {
	return len(u.Missing) != 0
}

// Clone returns an independent copy.
func (u FontUsage) Clone() FontUsage {
	out := FontUsage{Faces: append([]FaceUse(nil), u.Faces...)}
	if len(u.Missing) > 0 {
		out.Missing = make([]MissingFallback, len(u.Missing))
		for i, missing := range u.Missing {
			out.Missing[i] = missing
			out.Missing[i].TriedFaces = append([]string(nil), missing.TriedFaces...)
		}
	}
	return out
}

// Validate checks the structured face report.
func (u FontUsage) Validate() error {
	if len(u.Faces) == 0 && len(u.Missing) == 0 {
		return nil
	}
	seenFaces := make(map[string]struct{}, len(u.Faces))
	for i, face := range u.Faces {
		if !face.Source.IsValid() || face.Source.IsDetached() || face.Source.Len() == 0 {
			return fmt.Errorf("face use %d has an invalid source range", i)
		}
		if strings.TrimSpace(face.FaceID) == "" {
			return fmt.Errorf("face use %d has an empty face ID", i)
		}
		key := fmt.Sprintf("%d:%d:%s:%t:%t",
			face.Source.Start, face.Source.End, face.FaceID, face.Fallback, face.Missing)
		if _, exists := seenFaces[key]; exists {
			return fmt.Errorf("face use %d duplicates a prior record", i)
		}
		seenFaces[key] = struct{}{}
	}
	seenMissing := make(map[string]struct{}, len(u.Missing))
	for i, missing := range u.Missing {
		if !missing.Source.IsValid() || missing.Source.IsDetached() || missing.Source.Len() == 0 {
			return fmt.Errorf("missing fallback %d has an invalid source range", i)
		}
		if missing.Text == "" || !utf8.ValidString(missing.Text) {
			return fmt.Errorf("missing fallback %d has invalid text", i)
		}
		if strings.ContainsRune(missing.Text, '\x00') {
			return fmt.Errorf("missing fallback %d text contains NUL", i)
		}
		if len(missing.TriedFaces) == 0 {
			return fmt.Errorf("missing fallback %d has no tried faces", i)
		}
		tried := make(map[string]struct{}, len(missing.TriedFaces))
		for _, face := range missing.TriedFaces {
			if strings.TrimSpace(face) == "" {
				return fmt.Errorf("missing fallback %d has an empty tried face", i)
			}
			if _, duplicate := tried[face]; duplicate {
				return fmt.Errorf("missing fallback %d repeats tried face %q", i, face)
			}
			tried[face] = struct{}{}
		}
		key := missingKey(missing)
		if _, duplicate := seenMissing[key]; duplicate {
			return fmt.Errorf("missing fallback %d duplicates a prior record", i)
		}
		seenMissing[key] = struct{}{}
	}
	return nil
}

func missingKey(missing MissingFallback) string {
	return fmt.Sprintf("%d:%d:%s:%s",
		missing.Source.Start,
		missing.Source.End,
		missing.Text,
		strings.Join(missing.TriedFaces, "\x00"),
	)
}
