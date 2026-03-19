package vaultsync

import (
	"github.com/SomeBlackMagic/vault-manager/vault"
)

// ChangeType represents the type of change between local and remote state.
type ChangeType int

const (
	ChangeNone   ChangeType = iota // identical — no action
	ChangeAdd                      // local only → create in Vault
	ChangeModify                   // both exist, values differ → update Vault
	ChangeDelete                   // Vault only → delete from Vault
)

// Change represents a single difference between local and remote state.
type Change struct {
	Type       ChangeType
	Path       string
	LocalData  map[string]interface{} // nil if Vault-only
	RemoteData map[string]interface{} // nil if local-only
}

// ChangeSet holds all changes between local and remote state.
type ChangeSet struct {
	Changes []Change
}

// Counts returns the number of adds, modifies, and deletes in the ChangeSet.
func (cs ChangeSet) Counts() (adds, modifies, deletes int) {
	for _, c := range cs.Changes {
		switch c.Type {
		case ChangeAdd:
			adds++
		case ChangeModify:
			modifies++
		case ChangeDelete:
			deletes++
		}
	}
	return
}

// HasChanges returns true if there are any non-None changes.
func (cs ChangeSet) HasChanges() bool {
	adds, modifies, deletes := cs.Counts()
	return adds+modifies+deletes > 0
}

// LocalSecret represents a secret read from the local filesystem.
type LocalSecret struct {
	Path string
	Data map[string]interface{}
}

// VaultAccessor abstracts Vault operations for testability.
type VaultAccessor interface {
	Read(path string) (*vault.Secret, error)
	Write(path string, s *vault.Secret) error
	Delete(path string, opts vault.DeleteOpts) error
	List(path string) ([]string, error)
	ConstructSecrets(path string, opts vault.TreeOpts) (vault.Secrets, error)
}
