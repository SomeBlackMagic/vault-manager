package vaultsync

import (
	"log/slog"
	"os"
	"time"

	fmt "github.com/jhunt/go-ansi"

	"github.com/SomeBlackMagic/vault-manager/logging"
	"github.com/SomeBlackMagic/vault-manager/prompt"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

// Apply runs plan, displays output, prompts for confirmation, then applies changes.
// ChangeAdd/ChangeModify → PackMap(localData) to get map[string]string, then v.Write(path, secret)
// ChangeDelete → v.Delete(path, vault.DeleteOpts{})
// log receives diagnostic messages and may be nil.
func Apply(log *slog.Logger, v VaultAccessor, vaultPath, localDir string) error {
	log = logging.OrDiscard(log)
	start := time.Now()
	log.Debug("sync apply started", "vault_path", vaultPath, "local_dir", localDir)

	cs, err := Plan(log, v, vaultPath, localDir)
	if err != nil {
		return err
	}

	if !cs.HasChanges() {
		log.Debug("sync apply skipped: no changes")
		return nil
	}

	// Prompt for confirmation
	answer := prompt.Normal("\nDo you want to perform these actions? @C{(y/n)} ")
	if answer != "y" && answer != "yes" {
		fmt.Fprintf(os.Stderr, "Apply cancelled.\n")
		log.Debug("sync apply cancelled by user")
		return nil
	}

	adds, modifies, deletes := 0, 0, 0

	for _, c := range cs.Changes {
		switch c.Type {
		case ChangeAdd, ChangeModify:
			packed, err := PackMap(c.LocalData)
			if err != nil {
				return fmt.Errorf("packing data for %s: %s", c.Path, err)
			}
			secret := vault.NewSecret()
			for k, val := range packed {
				if err := secret.Set(k, val, false); err != nil {
					return fmt.Errorf("setting key %s for %s: %s", k, c.Path, err)
				}
			}
			log.Debug("applying change", logging.KeyPath, c.Path, "change", c.Type.String())
			if err := v.Write(c.Path, secret); err != nil {
				return fmt.Errorf("writing %s: %s", c.Path, err)
			}
			if c.Type == ChangeAdd {
				adds++
				fmt.Fprintf(os.Stderr, "@G{+} %s\n", c.Path)
			} else {
				modifies++
				fmt.Fprintf(os.Stderr, "@Y{~} %s\n", c.Path)
			}

		case ChangeDelete:
			log.Debug("applying change", logging.KeyPath, c.Path, "change", c.Type.String())
			if err := v.Delete(c.Path, vault.DeleteOpts{}); err != nil {
				return fmt.Errorf("deleting %s: %s", c.Path, err)
			}
			deletes++
			fmt.Fprintf(os.Stderr, "@R{-} %s\n", c.Path)
		}
	}

	fmt.Fprintf(os.Stderr, "\nApply complete! @G{%d} added, @Y{%d} changed, @R{%d} destroyed.\n", adds, modifies, deletes)
	log.Debug("sync apply completed",
		"adds", adds, "changes", modifies, "deletes", deletes,
		logging.KeyDuration, time.Since(start))
	return nil
}
