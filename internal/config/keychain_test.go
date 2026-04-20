package config

import (
	"testing"

	"github.com/google/uuid"
)

func TestKeychainKeyFormat(t *testing.T) {
	key := KeychainKey("prod-api-1")
	if key != "hangar:prod-api-1" {
		t.Fatalf("expected hangar:prod-api-1, got %s", key)
	}
}

func TestKeychainKeyFromUUID(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	key := KeychainKey(id.String())
	if key != "hangar:550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("expected hangar:<uuid>, got %s", key)
	}
}
