package main

import (
	"fmt"
	"runtime"

	"github.com/zalando/go-keyring"
)

const (
	keyringService  = "c1-report-builder"
	keyringIDKey    = "client-id"
	keyringSecretKey = "client-secret"
)

// credentialStore manages credential storage options.
type credentialStore struct{}

func newCredentialStore() *credentialStore {
	return &credentialStore{}
}

// platformName returns a user-friendly name for the credential store.
func (cs *credentialStore) platformName() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS Keychain"
	case "windows":
		return "Windows Credential Manager"
	default:
		return "System Keyring"
	}
}

// isAvailable checks if the system keyring is usable.
func (cs *credentialStore) isAvailable() bool {
	// Try a no-op read to see if keyring is accessible
	_, err := keyring.Get(keyringService, "__test__")
	// ErrNotFound means keyring works, just no value stored
	return err == keyring.ErrNotFound || err == nil
}

// save stores credentials in the system keyring.
func (cs *credentialStore) save(clientID, clientSecret string) error {
	if err := keyring.Set(keyringService, keyringIDKey, clientID); err != nil {
		return fmt.Errorf("failed to save Client ID: %w", err)
	}
	if err := keyring.Set(keyringService, keyringSecretKey, clientSecret); err != nil {
		return fmt.Errorf("failed to save Client Secret: %w", err)
	}
	return nil
}

// load retrieves credentials from the system keyring.
// Returns empty strings if not found.
func (cs *credentialStore) load() (string, string, error) {
	clientID, err := keyring.Get(keyringService, keyringIDKey)
	if err != nil {
		return "", "", nil // not found is not an error
	}
	clientSecret, err := keyring.Get(keyringService, keyringSecretKey)
	if err != nil {
		return "", "", nil
	}
	return clientID, clientSecret, nil
}

// clear removes stored credentials from the system keyring.
func (cs *credentialStore) clear() error {
	err1 := keyring.Delete(keyringService, keyringIDKey)
	err2 := keyring.Delete(keyringService, keyringSecretKey)
	if err1 != nil && err1 != keyring.ErrNotFound {
		return err1
	}
	if err2 != nil && err2 != keyring.ErrNotFound {
		return err2
	}
	return nil
}
