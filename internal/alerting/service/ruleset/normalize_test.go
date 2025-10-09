package ruleset

import "testing"

func TestNormalizeLabels(t *testing.T) {
	alias := map[string]string{"service_version": "version"}
	in := LabelMap{" Service ": " s3 ", "service_version": " V1 ", "empty": "  "}
	out := NormalizeLabels(in, alias)
	if out["service"] != "s3" || out["version"] != "V1" {
		t.Fatalf("unexpected normalize: %#v", out)
	}
	if _, ok := out["empty"]; ok {
		t.Fatalf("empty value should be removed: %#v", out)
	}
}

func TestCanonicalLabelKey(t *testing.T) {
	key1 := CanonicalLabelKey(LabelMap{"b": "2", "a": "1"})
	key2 := CanonicalLabelKey(LabelMap{"a": "1", "b": "2"})
	if key1 != key2 {
		t.Fatalf("keys should be equal: %s vs %s", key1, key2)
	}
}
