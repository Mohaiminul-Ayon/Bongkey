package security

import "testing"

// TestOrderToken verifies that guest secrets are distinct and only their own hash matches.
func TestOrderToken(t *testing.T) {
	first, firstHash, err := NewOrderToken()
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := NewOrderToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first == firstHash {
		t.Fatal("token must be random and stored only as a hash")
	}
	if !MatchesOrderToken(first, firstHash) {
		t.Fatal("issued token should match its hash")
	}
	for _, token := range []string{"", "invalid", second} {
		if MatchesOrderToken(token, firstHash) {
			t.Fatalf("unexpected access for token %q", token)
		}
	}
	if MatchesOrderToken(first, "") {
		t.Fatal("legacy orders without a hash must stay private")
	}
}
