package cargo

import (
	"sort"
	"strings"
)

// DeepReplace walks data, applying string replacements to every string it
// finds (including map keys). Non-string scalars pass through unchanged.
//
// Keys are applied in length-descending order so that a longer key whose
// prefix overlaps a shorter one wins. Without this, e.g. PROJECT_ROOT being
// a parent of CARGO_HOME would produce different output depending on map
// iteration order.
func DeepReplace(data any, replacements map[string]string) any {
	keys := sortedKeysLongestFirst(replacements)
	return deepReplace(data, replacements, keys)
}

func deepReplace(data any, replacements map[string]string, keys []string) any {
	switch v := data.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, val := range v {
			out[replaceWith(k, replacements, keys)] = deepReplace(val, replacements, keys)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = deepReplace(item, replacements, keys)
		}
		return out
	case string:
		return replaceWith(v, replacements, keys)
	default:
		return v
	}
}

// replaceString is kept for callers that don't pre-sort their keys.
func replaceString(s string, replacements map[string]string) string {
	return replaceWith(s, replacements, sortedKeysLongestFirst(replacements))
}

func replaceWith(s string, replacements map[string]string, keys []string) string {
	for _, k := range keys {
		s = strings.ReplaceAll(s, k, replacements[k])
	}
	return s
}

func sortedKeysLongestFirst(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	return keys
}
