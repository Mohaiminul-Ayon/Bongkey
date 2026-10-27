package security

import "testing"

// TestIdempotentOrderToken checks that retries reproduce only their own valid guest secret.
func TestIdempotentOrderToken(t *testing.T) {
	const secret = "synthetic-order-token-secret-at-least-32-characters"
	first, firstHash, err := IdempotentOrderToken(secret, "checkout-attempt-0001")
	if err != nil {
		t.Fatal(err)
	}
	retry, retryHash, err := IdempotentOrderToken(secret, "checkout-attempt-0001")
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := IdempotentOrderToken(secret, "checkout-attempt-0002")
	if err != nil {
		t.Fatal(err)
	}
	if first != retry || firstHash != retryHash || first == other || !MatchesOrderToken(first, firstHash) {
		t.Fatal("idempotent order token derivation is inconsistent")
	}
	for _, token := range []string{"", "invalid", other} {
		if MatchesOrderToken(token, firstHash) {
			t.Fatalf("unexpected access for token %q", token)
		}
	}
	if MatchesOrderToken(first, "") {
		t.Fatal("legacy orders without a hash must stay private")
	}
}
