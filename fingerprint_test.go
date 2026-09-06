package kmproto

import "testing"

func TestFingerprintKeys(t *testing.T) {
	// fingerprintKeys must be stable regardless of map iteration order.
	a := map[string]struct {
		Key string `json:"key"`
	}{
		"ed25519:a": {Key: "k1"},
		"ed25519:b": {Key: "k2"},
	}
	b := map[string]struct {
		Key string `json:"key"`
	}{
		"ed25519:b": {Key: "k2"},
		"ed25519:a": {Key: "k1"},
	}

	fa := fingerprintKeys(a)
	fb := fingerprintKeys(b)

	if fa == "" {
		t.Fatal("empty fingerprint for non-empty keys")
	}
	if fa != fb {
		t.Errorf("fingerprint differs across map order: %q != %q", fa, fb)
	}

	if got := fingerprintKeys(map[string]struct {
		Key string `json:"key"`
	}{}); got != "" {
		t.Errorf("empty keys should produce empty fingerprint, got %q", got)
	}
}
