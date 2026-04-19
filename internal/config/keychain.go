package config

import "github.com/zalando/go-keyring"

const keychainService = "hangar"

func KeychainKey(connName string) string {
	return "hangar:" + connName
}

func GetPassword(connName string) (string, error) {
	return keyring.Get(keychainService, connName)
}

func SetPassword(connName, password string) error {
	return keyring.Set(keychainService, connName, password)
}

func DeletePassword(connName string) error {
	return keyring.Delete(keychainService, connName)
}
