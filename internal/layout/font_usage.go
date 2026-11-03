package layout

import (
	"cmp"
	"slices"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) recordTextFontUsage(run TextRun, node *ir.Text) {
	if node == nil || len(run.Clusters) == 0 {
		return
	}
	runes := []rune(node.Text)
	for _, cluster := range run.Clusters {
		if cluster.FaceID == "" {
			continue
		}
		span := node.Span
		if len(node.Clusters) > 0 && cluster.Source.IsValid() && !cluster.Source.IsDetached() {
			span = cluster.Source
		}
		if !span.IsValid() || span.IsDetached() {
			continue
		}
		s.fontUsage.Faces = append(s.fontUsage.Faces, display.FaceUse{
			Source:   span,
			FaceID:   cluster.FaceID,
			Fallback: cluster.Fallback,
			Missing:  cluster.Missing,
		})
		if !cluster.Missing {
			continue
		}
		if cluster.RuneStart < 0 || cluster.RuneEnd <= cluster.RuneStart || cluster.RuneEnd > len(runes) {
			continue
		}
		tried := run.MissingFaces
		if len(tried) == 0 {
			tried = []string{cluster.FaceID}
		}
		s.fontUsage.Missing = append(s.fontUsage.Missing, display.MissingFallback{
			Source:     span,
			Text:       string(runes[cluster.RuneStart:cluster.RuneEnd]),
			TriedFaces: append([]string(nil), tried...),
		})
	}
}

func finalizeFontUsage(in display.FontUsage) display.FontUsage {
	out := in.Clone()
	slices.SortStableFunc(out.Faces, func(a, b display.FaceUse) int {
		if result := cmp.Compare(a.Source.Start, b.Source.Start); result != 0 {
			return result
		}
		if result := cmp.Compare(a.Source.End, b.Source.End); result != 0 {
			return result
		}
		if result := cmp.Compare(a.FaceID, b.FaceID); result != 0 {
			return result
		}
		if a.Fallback != b.Fallback {
			if !a.Fallback {
				return -1
			}
			return 1
		}
		if a.Missing != b.Missing {
			if !a.Missing {
				return -1
			}
			return 1
		}
		return 0
	})
	out.Faces = mergeFaceUses(out.Faces)

	slices.SortStableFunc(out.Missing, func(a, b display.MissingFallback) int {
		if result := cmp.Compare(a.Source.Start, b.Source.Start); result != 0 {
			return result
		}
		if result := cmp.Compare(a.Source.End, b.Source.End); result != 0 {
			return result
		}
		if result := cmp.Compare(a.Text, b.Text); result != 0 {
			return result
		}
		return slices.Compare(a.TriedFaces, b.TriedFaces)
	})
	out.Missing = mergeMissingFallbacks(out.Missing)
	return out
}

func mergeFaceUses(in []display.FaceUse) []display.FaceUse {
	if len(in) == 0 {
		return nil
	}
	out := make([]display.FaceUse, 0, len(in))
	for _, face := range in {
		last := len(out) - 1
		if last >= 0 && out[last] == face {
			continue
		}
		if last >= 0 &&
			out[last].Source.End == face.Source.Start &&
			out[last].FaceID == face.FaceID &&
			out[last].Fallback == face.Fallback &&
			out[last].Missing == face.Missing {
			out[last].Source.End = face.Source.End
			continue
		}
		out = append(out, face)
	}
	return out
}

func mergeMissingFallbacks(in []display.MissingFallback) []display.MissingFallback {
	if len(in) == 0 {
		return nil
	}
	out := make([]display.MissingFallback, 0, len(in))
	for _, missing := range in {
		last := len(out) - 1
		if last >= 0 && sameMissingFallback(out[last], missing) {
			continue
		}
		if last >= 0 &&
			out[last].Source.End == missing.Source.Start &&
			slices.Equal(out[last].TriedFaces, missing.TriedFaces) {
			out[last].Source.End = missing.Source.End
			out[last].Text += missing.Text
			continue
		}
		missing.TriedFaces = append([]string(nil), missing.TriedFaces...)
		out = append(out, missing)
	}
	return out
}

func sameMissingFallback(a, b display.MissingFallback) bool {
	return a.Source == b.Source &&
		a.Text == b.Text &&
		slices.Equal(a.TriedFaces, b.TriedFaces)
}
