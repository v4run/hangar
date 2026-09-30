package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/zalando/go-keyring"
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

func TestSetPasswordWrapsKeyringError(t *testing.T) {
	keyring.MockInitWithError(errors.New("The name is not activatable"))
	err := SetPassword("conn-1", "secret")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "keyring unavailable: The name is not activatable") {
		t.Fatalf("unexpected message: %s", msg)
	}
	if !strings.Contains(msg, "gnome-keyring") {
		t.Fatalf("expected hint in message: %s", msg)
	}
}

func TestDeletePasswordIgnoresNotFound(t *testing.T) {
	keyring.MockInit()
	if err := DeletePassword("never-saved"); err != nil {
		t.Fatalf("expected nil for missing entry, got %v", err)
	}
}

func TestDeletePasswordWrapsKeyringError(t *testing.T) {
	keyring.MockInitWithError(errors.New("boom"))
	err := DeletePassword("conn-1")
	if err == nil || !strings.HasPrefix(err.Error(), "keyring unavailable: boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetGetRoundTrip(t *testing.T) {
	keyring.MockInit()
	if err := SetPassword("conn-1", "secret"); err != nil {
		t.Fatal(err)
	}
	pw, err := GetPassword("conn-1")
	if err != nil || pw != "secret" {
		t.Fatalf("got %q, %v", pw, err)
	}
}
