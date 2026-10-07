package vault

import (
	"encoding/json"
	"fmt"

	"github.com/cloudfoundry-community/vaultkv"

	"github.com/SomeBlackMagic/vault-manager/logging"
)

// Read checks the Vault for a Secret at the specified path, and returns it.
// If there is nothing at that path, a nil *Secret will be returned, with no
// error.
func (v *Vault) Read(path string) (secret *Secret, err error) {
	path, key, version := ParsePath(path)
	v.Log().Debug("reading secret", logging.KeyPath, path, "key", key, "version", version)

	secret = NewSecret()

	raw := map[string]interface{}{}
	_, err = v.client.Get(path, &raw, &vaultkv.KVGetOpts{Version: uint(version)})
	if err != nil {
		if vaultkv.IsNotFound(err) {
			err = NewSecretNotFoundError(path)
		}
		return
	}

	if key != "" {
		val, found := raw[key]
		if !found {
			return nil, NewKeyNotFoundError(path, key)
		}
		raw = map[string]interface{}{key: val}
	}

	for k, v := range raw {
		if (key != "" && k == key) || key == "" {
			if s, ok := v.(string); ok {
				secret.data[k] = s
			} else {
				var b []byte
				b, err = json.Marshal(v)
				if err != nil {
					return
				}
				secret.data[k] = string(b)
			}
		}
	}

	return
}

// List returns the set of (relative) paths that are directly underneath
// the given path.  Intermediate path nodes are suffixed with a single "/",
// whereas leaf nodes (the secrets themselves) are not.
func (v *Vault) List(path string) (paths []string, err error) {
	path = Canonicalize(path)

	paths, err = v.client.List(path)
	if vaultkv.IsNotFound(err) {
		err = NewSecretNotFoundError(path)
	}

	return paths, err
}

// Write takes a Secret and writes it to the Vault at the specified path.
func (v *Vault) Write(path string, s *Secret) error {
	path, key, version := ParsePath(path)
	if key != "" {
		return fmt.Errorf("cannot write to paths in /path:key notation")
	}

	if version != 0 {
		return fmt.Errorf("cannot write to paths in /path^version notation")
	}

	if s.Empty() {
		return v.deleteIfPresent(path, DeleteOpts{})
	}

	// Only the number of keys is logged, never their values.
	v.Log().Debug("writing secret", logging.KeyPath, path, logging.KeyCount, len(s.data))
	_, err := v.client.Set(path, s.data, nil)
	if vaultkv.IsNotFound(err) {
		err = NewSecretNotFoundError(path)
	}

	return err
}
