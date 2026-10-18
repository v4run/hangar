package tui

import "testing"

// TestFlattenToastDropsNewlinesAndDupes replicates the exact
// ssh-keyscan failure the user hit: four copies of the same
// `getaddrinfo … not known` line separated by newlines plus the
// outer "exit status 1" preamble. The flattened result must be
// single-line and the duplicate noise collapsed.
func TestFlattenToastDropsNewlinesAndDupes(t *testing.T) {
	in := "key scan: ssh-keyscan: exit status 1 (getaddrinfo foo: nodename nor servname provided, or not known\n" +
		"getaddrinfo foo: nodename nor servname provided, or not known\n" +
		"getaddrinfo foo: nodename nor servname provided, or not known\n" +
		"getaddrinfo foo: nodename nor servname provided, or not known)"
	out := flattenToast(in)
	for _, bad := range []string{"\n", "\r"} {
		if contains(out, bad) {
			t.Fatalf("flattened toast still contains %q: %q", bad, out)
		}
	}
	// The duplicate copies should collapse to one occurrence.
	if count(out, "getaddrinfo foo") > 1 {
		t.Fatalf("duplicate lines not collapsed: %q", out)
	}
}

func TestFlattenToastHardCap(t *testing.T) {
	long := ""
	for i := 0; i < 500; i++ {
		long += "x"
	}
	out := flattenToast(long)
	if len(out) > 120 {
		t.Fatalf("flattened length %d exceeds cap 120: %q", len(out), out)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func count(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
			i += len(sub) - 1
		}
	}
	return n
}
