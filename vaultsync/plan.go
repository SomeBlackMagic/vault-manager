package vaultsync

import (
	"os"

	fmt "github.com/jhunt/go-ansi"

	"github.com/SomeBlackMagic/vault-manager/vault"
)

// Plan reads local state and remote state, computes ChangeSet, prints diff.
// Returns the ChangeSet for reuse in Apply.
func Plan(v VaultAccessor, vaultPath, localDir string) (ChangeSet, error) {
	// Read local state
	localSecrets, err := ReadLocalState(localDir)
	if err != nil {
		return ChangeSet{}, fmt.Errorf("reading local state from %s: %s", localDir, err)
	}

	// Fetch remote state
	remoteMap, err := fetchRemoteState(v, vaultPath)
	if err != nil {
		return ChangeSet{}, err
	}

	// Compute changes
	cs := ComputeChanges(localSecrets, remoteMap)

	// Print diff (skip unchanged)
	for _, c := range cs.Changes {
		if c.Type == ChangeNone {
			continue
		}
		fmt.Fprintf(os.Stderr, "%s", FormatDiff(c))
	}

	// Print summary
	if cs.HasChanges() {
		fmt.Fprintf(os.Stderr, "\n%s\n", FormatChangeSummary(cs))
	} else {
		fmt.Fprintf(os.Stderr, "No changes. Infrastructure is up-to-date.\n")
	}

	return cs, nil
}

// fetchRemoteState retrieves all secrets from Vault and returns them as expanded maps.
func fetchRemoteState(v VaultAccessor, vaultPath string) (map[string]map[string]interface{}, error) {
	secrets, err := v.ConstructSecrets(vaultPath, vault.TreeOpts{FetchKeys: true})
	if err != nil {
		return nil, fmt.Errorf("listing secrets at %s: %s", vaultPath, err)
	}

	remoteMap := make(map[string]map[string]interface{}, len(secrets))
	for _, entry := range secrets {
		if len(entry.Versions) == 0 {
			continue
		}
		latestData := entry.Versions[len(entry.Versions)-1].Data
		remoteMap[entry.Path] = secretToExpandedMap(latestData)
	}

	return remoteMap, nil
}
