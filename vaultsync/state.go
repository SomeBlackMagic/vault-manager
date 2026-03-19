package vaultsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SomeBlackMagic/vault-manager/vault"
)

// ReadLocalState walks localDir and parses all .json files.
// Returns list of LocalSecret with Path = vault path (relative to localDir, without .json suffix).
func ReadLocalState(localDir string) ([]LocalSecret, error) {
	var secrets []LocalSecret

	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}

		vaultPath := filePathToVaultPath(localDir, path)
		secrets = append(secrets, LocalSecret{
			Path: vaultPath,
			Data: m,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return secrets, nil
}

// WriteLocalSecret writes data as pretty-printed JSON to <localDir>/<vaultPath>.json.
// Creates intermediate directories as needed.
func WriteLocalSecret(localDir, vaultPath string, data map[string]interface{}) error {
	filePath := filepath.Join(localDir, vaultPath+".json")
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON for %s: %w", vaultPath, err)
	}
	b = append(b, '\n')

	if err := os.WriteFile(filePath, b, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", filePath, err)
	}
	return nil
}

// secretToExpandedMap extracts key-value pairs from vault.Secret,
// then runs ExpandMap to detect and expand JSON string values.
func secretToExpandedMap(s *vault.Secret) map[string]interface{} {
	flat := make(map[string]string)
	for _, k := range s.Keys() {
		flat[k] = s.Get(k)
	}
	return ExpandMap(flat)
}

// filePathToVaultPath converts a filesystem path to a vault path.
// Strips localDir prefix and .json suffix.
func filePathToVaultPath(localDir, filePath string) string {
	rel, _ := filepath.Rel(localDir, filePath)
	rel = strings.TrimSuffix(rel, ".json")
	// Normalize to forward slashes for vault paths
	return filepath.ToSlash(rel)
}
