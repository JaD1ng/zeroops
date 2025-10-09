package ruleset

import (
	"sort"
	"strings"
)

// NormalizeLabels returns a new LabelMap with keys normalized to lowercase, trimmed, aliases applied,
// empty values removed, and values trimmed. It does not mutate the input map.
// aliasMap maps alternative keys to canonical keys, e.g., "service_version" -> "version".
func NormalizeLabels(in LabelMap, aliasMap map[string]string) LabelMap {
	if len(in) == 0 {
		return LabelMap{}
	}
	result := make(LabelMap, len(in))
	for rawKey, rawVal := range in {
		key := strings.ToLower(strings.TrimSpace(rawKey))
		if key == "" {
			continue
		}
		if canonical, ok := aliasMap[key]; ok && strings.TrimSpace(canonical) != "" {
			key = strings.ToLower(strings.TrimSpace(canonical))
		}
		val := strings.TrimSpace(rawVal)
		if val == "" {
			continue
		}
		result[key] = val
	}
	return result
}

// CanonicalLabelKey returns a stable string representation of labels for use as a map key.
// It sorts keys and concatenates as key=value pairs separated by '|'.
// This ensures {a=1,b=2} and {b=2,a=1} produce identical keys.
func CanonicalLabelKey(labels LabelMap) string {
	if len(labels) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.Grow(len(keys) * 8)
	for i := 0; i < len(keys); i++ {
		if i > 0 {
			b.WriteByte('|')
		}
		k := keys[i]
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
	}
	return b.String()
}
