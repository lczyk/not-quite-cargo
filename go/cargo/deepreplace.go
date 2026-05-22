package cargo

import "strings"

// DeepReplace walks data, applying string replacements to every string it
// finds (including map keys). Non-string scalars pass through unchanged.
func DeepReplace(data any, replacements map[string]string) any {
	switch v := data.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, val := range v {
			out[replaceString(k, replacements)] = DeepReplace(val, replacements)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = DeepReplace(item, replacements)
		}
		return out
	case string:
		return replaceString(v, replacements)
	default:
		return v
	}
}

func replaceString(s string, replacements map[string]string) string {
	for old, new := range replacements {
		s = strings.ReplaceAll(s, old, new)
	}
	return s
}
