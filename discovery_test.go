package kmproto

import "testing"

func TestSplitServerName(t *testing.T) {
	tests := []struct {
		name        string
		wantHost    string
		wantPort    string
		wantHasPort bool
		wantErr     bool
	}{
		{"matrix.org", "matrix.org", "", false, false},
		{"matrix.org:8448", "matrix.org", "8448", true, false},
		{"1.2.3.4", "1.2.3.4", "", false, false},
		{"1.2.3.4:1234", "1.2.3.4", "1234", true, false},
		{"[::1]", "::1", "", false, false},
		{"[::1]:8448", "::1", "8448", true, false},
		{"1.2.3.4:1234:5678", "", "", false, true}, // too many colons
		{"[unclosed", "", "", false, true},         // malformed ipv6 literal
	}
	for _, tt := range tests {
		host, port, hasPort, err := splitServerName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("splitServerName(%q) err = %v, wantErr %v", tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if host != tt.wantHost || port != tt.wantPort || hasPort != tt.wantHasPort {
			t.Errorf("splitServerName(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.name, host, port, hasPort, tt.wantHost, tt.wantPort, tt.wantHasPort)
		}
	}
}
