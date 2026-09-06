package kmproto

import "testing"

func TestServerNameFromID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"!abc:example.org", "example.org"},
		{"#room:example.org", "example.org"},
		{"@user:example.org", "example.org"},
		{"$event:example.org", "example.org"},
		{"!abc:example.org:8448", "example.org:8448"},
		{"!abc:[::1]", "[::1]"},
		{"!abc:[::1]:8448", "[::1]:8448"},
		{"!domainless", ""}, // v4+ room ids may omit the server part
		{"no-colon", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := serverNameFromID(tt.in); got != tt.want {
			t.Errorf("serverNameFromID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
