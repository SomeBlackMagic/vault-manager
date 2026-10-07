package vaultsync

import (
	"log/slog"
	"os"
	"time"

	fmt "github.com/jhunt/go-ansi"

	"github.com/SomeBlackMagic/vault-manager/logging"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

// Plan reads local state and remote state, computes ChangeSet, prints diff.
// Returns the ChangeSet for reuse in Apply. log receives diagnostic messages
// (paths and counts only, never secret values) and may be nil.
func Plan(log *slog.Logger, v VaultAccessor, vaultPath, localDir string) (ChangeSet, error) {
	log = logging.OrDiscard(log)
	start := time.Now()
	log.Debug("sync plan started", "vault_path", vaultPath, "local_dir", localDir)

	// Read local state
	log.Debug("reading local state", "local_dir", localDir)
	localSecrets, err := ReadLocalState(localDir)
	if err != nil {
		return ChangeSet{}, fmt.Errorf("reading local state from %s: %s", localDir, err)
	}
	log.Debug("read local secrets", logging.KeyCount, len(localSecrets))

	// Fetch remote state
	remoteMap, err := fetchRemoteState(log, v, vaultPath)
	if err != nil {
		return ChangeSet{}, err
	}

	// Compute changes
	cs := ComputeChanges(localSecrets, remoteMap)
	for _, c := range cs.Changes {
		log.Debug("compared secret", logging.KeyPath, c.Path, "change", c.Type.String())
	}

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

	adds, modifies, deletes := cs.Counts()
	log.Debug("sync plan completed",
		"adds", adds, "changes", modifies, "deletes", deletes,
		logging.KeyDuration, time.Since(start))

	return cs, nil
}

// fetchRemoteState retrieves all secrets from Vault and returns them as expanded maps.
func fetchRemoteState(log *slog.Logger, v VaultAccessor, vaultPath string) (map[string]map[string]interface{}, error) {
	log.Debug("fetching remote secrets", "vault_path", vaultPath)
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
	log.Debug("fetched remote secrets", logging.KeyCount, len(remoteMap))

	return remoteMap, nil
}
