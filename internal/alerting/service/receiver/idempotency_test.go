package receiver

import (
	"testing"
	"time"
)

func TestBuildIdempotencyKey(t *testing.T) {
	a := AMAlert{
		Labels: KV{
			"service":         "test-service",
			"service_version": "v1.0.0",
		},
		StartsAt: time.Unix(0, 123).UTC(),
		Status:   "firing",
	}
	key := BuildIdempotencyKey(a)
	expected := "test-service|v1.0.0|1970-01-01T00:00:00.000000123Z|firing"
	if key != expected {
		t.Fatalf("unexpected key: %s, expected: %s", key, expected)
	}
}

func TestAlreadySeenAndMarkSeen(t *testing.T) {
	key := "k|t"
	if AlreadySeen(key) {
		t.Fatal("should not be seen initially")
	}
	MarkSeen(key)
	if !AlreadySeen(key) {
		t.Fatal("should be seen after MarkSeen")
	}
}
