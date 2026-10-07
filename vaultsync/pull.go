package vaultsync

import (
	"log/slog"
	"os"
	"time"

	fmt "github.com/jhunt/go-ansi"
	"github.com/mattn/go-isatty"

	"github.com/SomeBlackMagic/vault-manager/logging"
	"github.com/SomeBlackMagic/vault-manager/prompt"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

// Pull downloads all secrets at vaultPath to localDir as JSON files.
// For each secret:
//   - If local file doesn't exist: write it
//   - If local file exists and is identical: skip
//   - If local file exists and differs: show diff, prompt user (l=keep local, r=keep remote, s=skip)
//
// Creates localDir with os.MkdirAll if needed.
// log receives diagnostic messages and may be nil.
func Pull(log *slog.Logger, v VaultAccessor, vaultPath, localDir string) error {
	log = logging.OrDiscard(log)
	start := time.Now()
	log.Debug("sync pull started", "vault_path", vaultPath, "local_dir", localDir)

	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %s", localDir, err)
	}

	// Fetch all remote secrets
	log.Debug("fetching remote secrets", "vault_path", vaultPath)
	secrets, err := v.ConstructSecrets(vaultPath, vault.TreeOpts{FetchKeys: true})
	if err != nil {
		return fmt.Errorf("listing secrets at %s: %s", vaultPath, err)
	}
	log.Debug("fetched remote secrets", logging.KeyCount, len(secrets))

	// Read current local state
	localSecrets, err := ReadLocalState(localDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading local state: %s", err)
	}
	localMap := make(map[string]map[string]interface{}, len(localSecrets))
	for _, ls := range localSecrets {
		localMap[ls.Path] = ls.Data
	}
	log.Debug("read local secrets", logging.KeyCount, len(localMap))

	written, unchanged, kept := 0, 0, 0

	isTTY := isatty.IsTerminal(os.Stdin.Fd())

	for _, entry := range secrets {
		if len(entry.Versions) == 0 {
			continue
		}
		latestData := entry.Versions[len(entry.Versions)-1].Data
		remoteExpanded := secretToExpandedMap(latestData)

		localData, localExists := localMap[entry.Path]

		if !localExists {
			// New secret — just write it
			if err := WriteLocalSecret(localDir, entry.Path, remoteExpanded); err != nil {
				return err
			}
			written++
			log.Debug("wrote new local secret", logging.KeyPath, entry.Path)
			fmt.Fprintf(os.Stderr, "@G{+} %s\n", entry.Path)
			continue
		}

		if mapsEqual(localData, remoteExpanded) {
			// Identical — skip
			unchanged++
			log.Debug("local secret up to date", logging.KeyPath, entry.Path)
			continue
		}
		log.Debug("local secret differs from remote", logging.KeyPath, entry.Path, "interactive", isTTY)

		// Conflict — local differs from remote
		fmt.Fprintf(os.Stderr, "@Y{~} %s (local differs from remote)\n", entry.Path)

		change := Change{
			Type:       ChangeModify,
			Path:       entry.Path,
			LocalData:  localData,
			RemoteData: remoteExpanded,
		}
		fmt.Fprintf(os.Stderr, "%s", FormatDiff(change))

		if !isTTY {
			// Non-interactive: keep remote (safe default)
			if err := WriteLocalSecret(localDir, entry.Path, remoteExpanded); err != nil {
				return err
			}
			written++
			fmt.Fprintf(os.Stderr, "  (non-interactive: keeping remote)\n")
			continue
		}

		for {
			answer := prompt.Normal("  Keep @C{(l)}ocal, @C{(r)}emote, or @C{(s)}kip? ")
			switch answer {
			case "l":
				kept++
				fmt.Fprintf(os.Stderr, "  Keeping local\n")
				goto nextSecret
			case "r":
				if err := WriteLocalSecret(localDir, entry.Path, remoteExpanded); err != nil {
					return err
				}
				written++
				fmt.Fprintf(os.Stderr, "  Keeping remote\n")
				goto nextSecret
			case "s":
				kept++
				fmt.Fprintf(os.Stderr, "  Skipping\n")
				goto nextSecret
			default:
				fmt.Fprintf(os.Stderr, "  Please enter 'l', 'r', or 's'\n")
			}
		}
	nextSecret:
	}

	log.Debug("sync pull completed",
		"written", written, "unchanged", unchanged, "kept_local", kept,
		logging.KeyDuration, time.Since(start))
	return nil
}
