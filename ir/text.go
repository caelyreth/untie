package ir

import (
	"unicode/utf8"

	"github.com/caelyreth/untie/source"
)

// NormalizeTextClusters validates a text cluster map and returns a copy in
// canonical order. Adjacent entries whose source ranges overlap are coalesced,
// allowing macro-expanded text to retain a useful invocation-level map without
// emitting overlapping editor hits.
//
// Nil is returned when the input does not completely partition text at UTF-8
// boundaries, contains an invalid source range, or has source ranges that
// cannot be ordered without losing information. Callers must leave the Text
// map empty in that case and rely on the node-level hit fallback.
func NormalizeTextClusters(text string, clusters []TextClusterSource) []TextClusterSource {
	if text == "" || len(clusters) == 0 {
		return nil
	}
	out := make([]TextClusterSource, 0, len(clusters))
	textEnd := 0
	for _, cluster := range clusters {
		if cluster.TextStart != textEnd || cluster.TextEnd <= cluster.TextStart ||
			cluster.TextEnd > len(text) || cluster.Source.IsDetached() ||
			!cluster.Source.IsValid() || cluster.Source.End <= cluster.Source.Start ||
			!utf8.RuneStart(text[cluster.TextStart]) ||
			(cluster.TextEnd < len(text) && !utf8.RuneStart(text[cluster.TextEnd])) {
			return nil
		}
		if len(out) > 0 && cluster.Source.Start < out[len(out)-1].Source.End {
			if cluster.Source.Start < out[len(out)-1].Source.Start {
				return nil
			}
			out[len(out)-1].TextEnd = cluster.TextEnd
			out[len(out)-1].Source = source.Merge(out[len(out)-1].Source, cluster.Source)
			for len(out) > 1 && out[len(out)-1].Source.Start < out[len(out)-2].Source.End {
				previous := out[len(out)-2]
				current := out[len(out)-1]
				previous.TextEnd = current.TextEnd
				previous.Source = source.Merge(previous.Source, current.Source)
				out = out[:len(out)-1]
				out[len(out)-1] = previous
			}
		} else {
			out = append(out, cluster)
		}
		textEnd = cluster.TextEnd
	}
	if textEnd != len(text) {
		return nil
	}
	return out
}
