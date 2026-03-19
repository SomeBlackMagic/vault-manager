package cmd

import (
	"github.com/SomeBlackMagic/vault-manager/app"
	"github.com/SomeBlackMagic/vault-manager/rc"
	"github.com/SomeBlackMagic/vault-manager/vaultsync"
)

func registerSyncCommands(r *app.Runner, opt *Options) {
	r.Dispatch("sync", &app.Help{
		Summary: "Manage secrets via local filesystem (pull/plan/apply)",
		Usage:   "vault-manager sync <pull|plan|apply> VAULT-PATH LOCAL-DIR",
		Type:    app.AdministrativeCommand,
		Description: `
Manage Vault secrets using a Terraform-style pull/plan/apply workflow.

Secrets are stored locally as JSON files, one file per Vault path.
String values that contain embedded JSON objects or arrays are expanded
into nested structures for human-readable editing, and re-packed on apply.

Subcommands:

    pull    Download all secrets from Vault to local JSON files.
            Prompts on conflict when a local file differs from remote.

    plan    Show what changes would be applied (local vs remote diff).
            Does not modify anything.

    apply   Apply local changes to Vault (after showing a plan and
            prompting for confirmation).

`,
	}, func(command string, args ...string) error {
		r.ExitWithUsage("sync")
		return nil
	})

	r.Dispatch("sync pull", &app.Help{
		Summary: "Download Vault secrets to local JSON files",
		Usage:   "vault-manager sync pull VAULT-PATH LOCAL-DIR",
		Type:    app.NonDestructiveCommand,
		Description: `
Download all secrets under VAULT-PATH to LOCAL-DIR as JSON files.

Each Vault secret path maps to a corresponding .json file under LOCAL-DIR.
For example, secret/app/db → LOCAL-DIR/secret/app/db.json

String values that contain valid JSON objects or arrays (starting with
{ or [) are automatically expanded into nested JSON for easier editing.

Conflict handling:
  - Local file missing:  write remote version
  - Local == remote:     skip (no change)
  - Local differs:       show diff, prompt for (l)ocal / (r)emote / (s)kip
  - Non-TTY (piped):     automatically keep remote

`,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		if len(args) != 2 {
			r.ExitWithUsage("sync pull")
		}
		v := app.Connect(true)
		return vaultsync.Pull(v, args[0], args[1])
	})

	r.Dispatch("sync plan", &app.Help{
		Summary: "Show what changes would be applied to Vault",
		Usage:   "vault-manager sync plan VAULT-PATH LOCAL-DIR",
		Type:    app.NonDestructiveCommand,
		Description: `
Compare local JSON files in LOCAL-DIR against secrets in Vault at VAULT-PATH
and display a diff showing what would change on apply.

Does not modify Vault or local files.

Output symbols:
  @G{+}  Secret exists locally but not in Vault (would be created)
  @Y{~}  Secret exists in both but differs (would be updated)
  @R{-}  Secret exists in Vault but not locally (would be deleted)
     No symbol: secret is identical, no change

For modified secrets, shows field-level diffs. Values that are nested JSON
objects display granular field changes instead of the full blob.

`,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		if len(args) != 2 {
			r.ExitWithUsage("sync plan")
		}
		v := app.Connect(true)
		_, err := vaultsync.Plan(v, args[0], args[1])
		return err
	})

	r.Dispatch("sync apply", &app.Help{
		Summary: "Apply local changes to Vault",
		Usage:   "vault-manager sync apply VAULT-PATH LOCAL-DIR",
		Type:    app.DestructiveCommand,
		Description: `
Compare local JSON files in LOCAL-DIR against secrets in Vault at VAULT-PATH,
display the plan, prompt for confirmation, then apply all changes.

  @G{+} Created:  writes new secret to Vault
  @Y{~} Modified: updates existing secret in Vault
  @R{-} Deleted:  removes secret from Vault

Nested JSON objects in local files are re-serialized to compact JSON
strings before writing, so Vault always receives flat key-value pairs.

`,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		if len(args) != 2 {
			r.ExitWithUsage("sync apply")
		}
		v := app.Connect(true)
		return vaultsync.Apply(v, args[0], args[1])
	})

}
