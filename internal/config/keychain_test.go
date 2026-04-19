package config

import (
	"testing"
)

func TestKeychainKeyFormat(t *testing.T) {
	key := KeychainKey("prod-api-1")
	if key != "hangar:prod-api-1" {
		t.Fatalf("expected hangar:prod-api-1, got %s", key)
	}
}
