package config

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const keychainService = "hangar"

func KeychainKey(connName string) string {
	return "hangar:" + connName
}

func GetPassword(connName string) (string, error) {
	pw, err := keyring.Get(keychainService, connName)
	if err != nil {
		return "", wrapKeyringErr(err)
	}
	return pw, nil
}

func SetPassword(connName, password string) error {
	return wrapKeyringErr(keyring.Set(keychainService, connName, password))
}

// DeletePassword removes the stored password. A missing entry is not an
// error: callers delete unconditionally when a form is saved without one.
func DeletePassword(connName string) error {
	err := keyring.Delete(keychainService, connName)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return wrapKeyringErr(err)
}

// wrapKeyringErr turns go-keyring's terse backend errors ("The name is not
// activatable", "failed to unlock correct collection ...") into something
// that points the user at the fix. ErrNotFound is passed through unchanged
// so callers can still detect it.
func wrapKeyringErr(err error) error {
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return fmt.Errorf("keyring unavailable: %w (is a Secret Service such as gnome-keyring running and unlocked?)", err)
}
