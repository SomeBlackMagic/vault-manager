package cmd

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/cloudfoundry-community/vaultkv"
	fmt "github.com/jhunt/go-ansi"
	"github.com/SomeBlackMagic/vault-manager/app"
	"github.com/SomeBlackMagic/vault-manager/rc"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

func registerAdminCommands(r *app.Runner, opt *Options) {
	r.Dispatch("status", &app.Help{
		Summary: "Print the status of the current target's backend nodes",
		Type:    app.AdministrativeCommand,
		Usage:   "vault-manager status",
		Description: `
Returns the seal status of each node in the Vault cluster.

If strongbox is configured for this target, then strongbox is queried for seal
status of all nodes in the cluster. If strongbox is disabled for the target,
the /sys/health endpoint is queried for the target box to return the health of
just this Vault instance.

The following options are recognized:

	-e, --err-sealed  Causes vault-manager to exit with a non-zero code if any of the
	                  queried Vaults are sealed.
	`,
	}, func(command string, args ...string) error {
		cfg := rc.Apply(opt.UseTarget)
		v := app.Connect(false)

		type status struct {
			addr   string
			sealed bool
		}

		var statuses []status

		if cfg.HasStrongbox() {
			st, err := v.Strongbox()
			if err != nil {
				return fmt.Errorf("%w; are you targeting a `vault-manager' installation?", err)
			}

			for addr, state := range st {
				statuses = append(statuses, status{addr, state == "sealed"})
			}
		} else {
			if err := v.SetURL(cfg.URL()); err != nil {
				return err
			}
			isSealed, err := v.Sealed()
			if err != nil {
				return err
			}

			statuses = append(statuses, status{cfg.URL(), isSealed})
		}

		var hasSealed bool

		for _, s := range statuses {
			if s.sealed {
				hasSealed = true
				fmt.Printf("@R{%s is sealed}\n", s.addr)
			} else {
				fmt.Printf("@G{%s is unsealed}\n", s.addr)
			}
		}

		if opt.Status.ErrorIfSealed && hasSealed {
			return fmt.Errorf("There are sealed Vaults")
		}

		return nil
	})

	r.Dispatch("local", &app.Help{
		Summary: "Run a local vault",
		Usage:   "vault-manager local (--memory|--file path/to/dir) [--as name] [--port port]",
		Description: `
Spins up a new Vault instance.

By default, an unused port between 8201 and 9999 (inclusive) will be selected as
the Vault listening port. You may manually specify a port with the -p/--port
flag.

The new Vault will be initialized with a single seal key, targeted with
a catchy name, authenticated by the new root token, and populated with a
secret/handshake!

If you just need a transient Vault for testing or experimentation, and
don't particularly care about the contents of the Vault, specify the
--memory/-m flag and get an in-memory backend.

If, on the other hand, you want to keep the Vault around, possibly
spinning it down when not in use, specify the --file/-f flag, and give it
the path to a directory to use for the file backend.  The files created
by the mechanism will be encrypted.  You will be given the seal key for
subsequent activations of the Vault.
`,
		Type: app.AdministrativeCommand,
	}, func(command string, args ...string) error {
		if !opt.Local.Memory && opt.Local.File == "" {
			return fmt.Errorf("Please specify either --memory or --file <path>")
		}
		if opt.Local.Memory && opt.Local.File != "" {
			return fmt.Errorf("Please specify either --memory or --file <path>, but not both")
		}

		var port int
		if opt.Local.Port != 0 {
			port = opt.Local.Port
		} else {
			for port = 8201; port < 9999; port++ {
				conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
				if err != nil {
					break
				}
				conn.Close()
			}
		}

		f, err := os.CreateTemp("", "kazoo")
		if err != nil {
			return err
		}
		fmt.Fprintf(f, `# vault-manager local config
disable_mlock = true

listener "tcp" {
  address     = "127.0.0.1:%d"
  tls_disable = 1
}
`, port)

		//the "storage" configuration key was once called "backend"
		storageKey := "storage"
		cmd := exec.Command("vault", "version")
		versionOutput, err := cmd.CombinedOutput()
		if err == nil {
			matches := regexp.MustCompile("v([0-9]+)\\.([0-9]+)").FindSubmatch(versionOutput)
			if len(matches) >= 3 {
				major, err := strconv.ParseUint(string(matches[1]), 10, 64)
				if err != nil {
					goto doneVersionCheck
				}
				minor, err := strconv.ParseUint(string(matches[2]), 10, 64)
				if err != nil {
					goto doneVersionCheck
				}

				//if version < 0.8.0
				if major == 0 && minor < 8 {
					storageKey = "backend"
				}
			}
		} else {
			return fmt.Errorf("@R{Vault is not currently installed or located in $PATH}")
		}
	doneVersionCheck:

		keys := make([]string, 0)
		if opt.Local.Memory {
			fmt.Fprintf(f, "%s \"inmem\" {}\n", storageKey)
		} else {
			opt.Local.File = filepath.ToSlash(opt.Local.File)
			fmt.Fprintf(f, "%s \"file\" { path = \"%s\" }\n", storageKey, opt.Local.File)
			if _, err := os.Stat(opt.Local.File); err == nil || !os.IsNotExist(err) {
				keys = append(keys, app.Pr("Unseal Key", false, true))
			}
		}

		echan := make(chan error)
		cmd = exec.Command("vault", "server", "-config", f.Name())
		cmd.Start()
		go func() {
			echan <- cmd.Wait()
		}()
		signal.Ignore(syscall.SIGINT)

		die := func(err error) {
			if err != nil {
				fmt.Fprintf(os.Stderr, "@R{!! %s}\n", err)
			}
			fmt.Fprintf(os.Stderr, "@Y{shutting down the Vault...}\n")
			if err := cmd.Process.Kill(); err != nil {
				fmt.Fprintf(os.Stderr, "@R{NOTE: Unable to terminate the Vault process.}\n")
				fmt.Fprintf(os.Stderr, "@R{      You may have some environmental cleanup to do.}\n")
				fmt.Fprintf(os.Stderr, "@R{      Apologies.}\n")
			}
			os.Exit(1)
		}

		cfg := rc.Apply("")
		name := opt.Local.As
		if name == "" {
			name = app.RandomName()
			var n int
			for n = 15; n > 0; n-- {
				if existing, _ := cfg.Vault(name); existing == nil {
					break
				}
				name = app.RandomName()
			}
			if n == 0 {
				die(fmt.Errorf("I was unable to come up with a cool name for your local Vault.  Please try naming it with --as"))
			}
		} else {
			if existing, _ := cfg.Vault(name); existing != nil {
				die(fmt.Errorf("You already have '%s' as a Vault target", name))
			}
		}
		previous := cfg.Current

		cfg.SetTarget(name, rc.Vault{
			URL:         fmt.Sprintf("http://127.0.0.1:%d", port),
			SkipVerify:  false,
			NoStrongbox: true,
		})
		cfg.Write()

		rc.Apply("")
		v := app.Connect(false)

		const maxStartupWait = 5 * time.Second
		const betweenChecksWait = 250 * time.Millisecond
		startupCheckBeginTime := time.Now()
		for {
			_, err := v.Sealed()
			if err == nil {
				break
			}

			if time.Since(startupCheckBeginTime) > maxStartupWait {
				die(fmt.Errorf("Timed out waiting for Vault to begin listening: %w", err))
			}

			time.Sleep(betweenChecksWait)
		}

		token := ""
		if len(keys) == 0 {
			keys, _, err = v.Init(1, 1)
			if err != nil {
				die(fmt.Errorf("Unable to initialize the new (temporary) Vault: %w", err))
			}
		}

		if err = v.Unseal(keys); err != nil {
			die(fmt.Errorf("Unable to unseal the new (temporary) Vault: %w", err))
		}
		token, err = v.NewRootToken(keys)
		if err != nil {
			die(fmt.Errorf("Unable to generate a new root token: %w", err))
		}

		cfg.SetToken(token)
		os.Setenv("VAULT_TOKEN", token)
		cfg.Write()
		v = app.Connect(true)

		exists, err := v.MountExists("secret")
		if err != nil {
			return fmt.Errorf("Could not list mounts: %w", err)
		}

		if !exists {
			err := v.AddMount("secret", 2)
			if err != nil {
				return fmt.Errorf("Could not add `secret' mount: %w", err)
			}
			fmt.Printf("vault-manager has mounted the @C{secret} backend\n\n")
		}

		s := vault.NewSecret()
		s.Set("knock", "knock", false)
		v.Write("secret/handshake", s)

		if !opt.Quiet {
			fmt.Fprintf(os.Stderr, "Now targeting (temporary) @Y{%s} at @C{%s}\n", cfg.Current, cfg.URL())
			if opt.Local.Memory {
				fmt.Fprintf(os.Stderr, "@R{This Vault is MEMORY-BACKED!}\n")
				fmt.Fprintf(os.Stderr, "If you want to @Y{retain your secrets} be sure to @C{vault-manager export}.\n")
			} else {
				fmt.Fprintf(os.Stderr, "Storing data (encrypted) in @G{%s}\n", opt.Local.File)
				fmt.Fprintf(os.Stderr, "Your Vault Seal Key is @M{%s}\n", keys[0])
			}
			fmt.Fprintf(os.Stderr, "Ctrl-C to shut down the Vault\n")
		}

		err = <-echan
		fmt.Fprintf(os.Stderr, "Vault terminated normally, cleaning up...\n")
		cfg = rc.Apply("")
		if cfg.Current == name {
			cfg.Current = ""
			if _, found, _ := cfg.Find(previous); found {
				cfg.Current = previous
			}
		}
		delete(cfg.Vaults, name)
		cfg.Write()
		return err
	})

	r.Dispatch("init", &app.Help{
		Summary: "Initialize a new vault",
		Usage:   "vault-manager init [--keys #] [--threshold #] [--single] [--json] [--no-mount] [--sealed]",
		Description: `
Initializes a brand new Vault backend, generating new seal keys, and an
initial root token.  This information will be printed out, so that you
can save it somewhere secure (encrypted drive, password manager, etc.)

By default, Vault is initialized with 5 unseal keys, 3 of which are
required to unseal the Vault after a restart.  You can adjust this via
the --keys and --threshold options.  The --single option is a shortcut
for specifying a single key and a threshold of 1.

Once the Vault is initialized, vault-manager will unseal it automatically, using
the newly minted seal keys, unless you pass it the --sealed option.
The root token will also be stored in the ~/.vault-managerrc file, saving you the
trouble of calling 'vault-manager auth token' yourself.

The --json flag causes 'vault-manager init' to print out the seal keys and initial
root token in a machine-friendly JSON format, that looks like this:

    {
      "root_token": "05f28556-db0a-f76f-3c26-40de20f28cee"
      "seal_keys": [
        "jDuvcXg7s4QnjHjwN9ydSaFtoMj8YZWrO8hRFWT2PoqT",
        "XiE5cq0+AsUcK8EK8GomCsMdylixwWa8tM2L991OHcry",
        "F9NbroyispQTCMHBWBD5+lYxMEms5hntwsrxcdZx1+3w",
        "3scP3yIdfLv9mr0YbxZRClpPNSf5ohVpWmxrpRQ/a9JM",
        "NosOaAjZzvcdHKBvtaqLDRwWSG6/XkLwgZHvnIvAhOC5"
      ]
    }

This can be used to automate the setup of Vaults for test/dev purposes,
which can be quite handy.

By default, the seal keys will also be stored in the Vault itself,
unless you specify the --no-persist flag.  They will be written to
secret/vault/seal/keys, as key1, key2, ... keyN. Note that if
--sealed is also set, this option is ignored (since the Vault will
remain sealed).

In more recent versions of Vault, the "secret" mount is not mounted
by default. Safe will ensure that the mount is mounted anyway unless
the --no-mount option is given. The flag will not unmount an existing
secret mount in versions of Vault which mount "secret" by default.
Note that if --sealed is also set, this option is ignored (since the
Vault will remain sealed).

`,
		Type: app.AdministrativeCommand,
	}, func(command string, args ...string) error {
		cfg := rc.Apply(opt.UseTarget)
		v := app.Connect(false)

		if opt.Init.NKeys == 0 {
			opt.Init.NKeys = 5
		}
		if opt.Init.Threshold == 0 {
			if opt.Init.NKeys > 3 {
				opt.Init.Threshold = opt.Init.NKeys - 2
			} else {
				opt.Init.Threshold = opt.Init.NKeys
			}
		}

		if opt.Init.Single {
			opt.Init.NKeys = 1
			opt.Init.Threshold = 1
		}

		/* initialize the vault */
		keys, token, err := v.Init(opt.Init.NKeys, opt.Init.Threshold)
		if err != nil {
			return err
		}

		if token == "" {
			panic("token was nil")
		}

		/* auth with the new root token, transparently */
		cfg.SetToken(token)
		if err := cfg.Write(); err != nil {
			return err
		}
		os.Setenv("VAULT_TOKEN", token)
		v = app.Connect(true)

		/* be nice to the machines and machine-like intelligences */
		if opt.Init.JSON {
			out := struct {
				Keys  []string `json:"seal_keys"`
				Token string   `json:"root_token"`
			}{
				Keys:  keys,
				Token: token,
			}

			b, err := json.MarshalIndent(&out, "", "  ")
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", string(b))
		} else {
			for i, key := range keys {
				fmt.Printf("Unseal Key #%d: @G{%s}\n", i+1, key)
			}
			fmt.Printf("Initial Root Token: @M{%s}\n", token)
			fmt.Printf("\n")
			if opt.Init.NKeys == 1 {
				fmt.Printf("Vault initialized with a single key. Please securely distribute it.\n")
				fmt.Printf("When the Vault is re-sealed, restarted, or stopped, you must provide\n")
				fmt.Printf("this key to unseal it again.\n")
				fmt.Printf("\n")
				fmt.Printf("Vault does not store the master key. Without the above unseal key,\n")
				fmt.Printf("your Vault will remain permanently sealed.\n")

			} else if opt.Init.NKeys == opt.Init.Threshold {
				fmt.Printf("Vault initialized with %d keys. Please securely distribute the\n", opt.Init.NKeys)
				fmt.Printf("above keys. When the Vault is re-sealed, restarted, or stopped,\n")
				fmt.Printf("you must provide all of these keys to unseal it again.\n")
				fmt.Printf("\n")
				fmt.Printf("Vault does not store the master key. Without all %d of the keys,\n", opt.Init.Threshold)
				fmt.Printf("your Vault will remain permanently sealed.\n")

			} else {
				fmt.Printf("Vault initialized with %d keys and a key threshold of %d. Please\n", opt.Init.NKeys, opt.Init.Threshold)
				fmt.Printf("securely distribute the above keys. When the Vault is re-sealed,\n")
				fmt.Printf("restarted, or stopped, you must provide at least %d of these keys\n", opt.Init.Threshold)
				fmt.Printf("to unseal it again.\n")
				fmt.Printf("\n")
				fmt.Printf("Vault does not store the master key. Without at least %d keys,\n", opt.Init.Threshold)
				fmt.Printf("your Vault will remain permanently sealed.\n")
			}

			fmt.Printf("\n")
		}

		if !opt.Init.Sealed {
			addrs := []string{}
			gotStrongbox := false
			if cfg.HasStrongbox() {
				if st, err := v.Strongbox(); err == nil {
					gotStrongbox = true
					for addr := range st {
						addrs = append(addrs, addr)
					}
				}
			}
			if !gotStrongbox {
				addrs = append(addrs, v.Client().Client.VaultURL.String())
			}

			for _, addr := range addrs {
				if err := v.SetURL(addr); err != nil {
					fmt.Fprintf(os.Stderr, "!!! unable to set URL for vault (at %s): %s\n", addr, err)
					continue
				}
				if err := v.Unseal(keys); err != nil {
					fmt.Fprintf(os.Stderr, "!!! unable to unseal newly-initialized vault (at %s): %s\n", addr, err)
				}
			}

			//Make a best attempt to wait until Vault has figured out which node should be the master.
			// This doesn't error out if no master comes forward, as there may be a cluster but no
			// Strongbox. In that case, it may error later, but we've done what we can.
			const maxAttempts = 5
			const waitInterval = 500 * time.Millisecond
			var currentAttempt int
		waitMaster:
			for currentAttempt < maxAttempts {
				for _, addr := range addrs {
					if err := v.SetURL(addr); err != nil {
						continue
					}
					if err := v.Client().Client.Health(false); err == nil {
						break waitMaster
					}
				}
				currentAttempt++
				time.Sleep(waitInterval)
			}

			if !opt.Init.NoMount {
				exists, err := v.MountExists("secret")
				if err != nil {
					return fmt.Errorf("Could not list mounts: %w", err)
				}

				if !exists {
					err := v.AddMount("secret", 2)
					if err != nil {
						return fmt.Errorf("Could not add `secret' mount: %w", err)
					}

					if !opt.Init.JSON {
						fmt.Printf("vault-manager has mounted the @C{secret} backend\n")
					}
				}
			}

			/* write secret/handshake, just for fun */
			s := vault.NewSecret()
			s.Set("knock", "knock", false)
			v.Write("secret/handshake", s)

			if !opt.Init.JSON {
				fmt.Printf("vault-manager has unsealed the Vault for you, and written a test value\n")
				fmt.Printf("at @C{secret/handshake}.\n\n")
			}

			/* write seal keys to the vault */
			if opt.Init.Persist {
				v.SaveSealKeys(keys)
				if !opt.Init.JSON {
					fmt.Printf("vault-manager has written the unseal keys at @C{secret/vault/seal/keys}\n")
				}
			}
		} else {
			if !opt.Init.JSON {
				fmt.Printf("Your Vault has been left sealed.\n")
			}
		}

		if !opt.Init.JSON {
			fmt.Printf("\n")
			fmt.Printf("You have been automatically authenticated to the Vault with the\n")
			fmt.Printf("initial root token.  Be vault-manager out there!\n")
			fmt.Printf("\n")
		}

		return nil
	})

	r.Dispatch("unseal", &app.Help{
		Summary: "Unseal the current target",
		Usage:   "vault-manager unseal",
		Type:    app.AdministrativeCommand,
	}, func(command string, args ...string) error {
		cfg := rc.Apply(opt.UseTarget)
		v := app.Connect(false)

		var addrs []string
		if cfg.HasStrongbox() {
			st, err := v.Strongbox()
			if err != nil {
				return fmt.Errorf("%w; are you targeting a `vault-manager' installation?", err)
			}

			for addr, state := range st {
				if state == "sealed" {
					addrs = append(addrs, addr)
				}
			}
		} else {
			if err := v.SetURL(cfg.URL()); err != nil {
				return err
			}
			isSealed, err := v.Sealed()
			if err != nil {
				return err
			}

			if isSealed {
				addrs = append(addrs, cfg.URL())
			}
		}

		if len(addrs) == 0 {
			fmt.Printf("@C{all vaults are already unsealed!}\n")
			return nil
		}

		if err := v.SetURL(addrs[0]); err != nil {
			return err
		}
		nkeys, err := v.SealKeys()
		if err != nil {
			return err
		}

		fmt.Printf("You need %d key(s) to unseal the vaults.\n\n", nkeys)
		keys := make([]string, nkeys)

		for i := 0; i < nkeys; i++ {
			keys[i] = app.Pr(fmt.Sprintf("Key #%d", i+1), false, true)
		}

		for _, addr := range addrs {
			fmt.Printf("unsealing @G{%s}...\n", addr)
			if err := v.SetURL(addr); err != nil {
				return err
			}
			err = v.Unseal(keys)
			if err != nil {
				return err
			}
		}

		return nil
	})

	r.Dispatch("seal", &app.Help{
		Summary: "Seal the current target",
		Usage:   "vault-manager seal",
		Type:    app.AdministrativeCommand,
	}, func(command string, args ...string) error {
		cfg := rc.Apply(opt.UseTarget)
		v := app.Connect(true)

		var toSeal []string
		if cfg.HasStrongbox() {
			st, err := v.Strongbox()
			if err != nil {
				return fmt.Errorf("%w; are you targeting a `vault-manager' installation?", err)
			}

			for addr, state := range st {
				if state == "unsealed" {
					toSeal = append(toSeal, addr)
				}
			}
		} else {
			if err := v.SetURL(cfg.URL()); err != nil {
				return err
			}
			isSealed, err := v.Sealed()
			if err != nil {
				return nil
			}
			if !isSealed {
				toSeal = append(toSeal, cfg.URL())
			}
		}

		if len(toSeal) == 0 {
			fmt.Printf("@C{all vaults are already sealed!}\n")
		}

		consecutiveFailures := 0
		const maxFailures = 10
		const attemptInterval = 500 * time.Millisecond

		for len(toSeal) > 0 {
			for i, addr := range toSeal {
				if err := v.SetURL(addr); err != nil {
					continue
				}
				err := v.Client().Client.Health(false)
				if err != nil {
					if vaultkv.IsErrStandby(err) {
						continue
					}

					return err
				}

				sealed, err := v.Seal()
				if err != nil {
					return err
				}

				if sealed {
					fmt.Printf("sealed @G{%s}...\n", addr)
					//Remove sealed Vault from list
					toSeal[i], toSeal[len(toSeal)-1] = toSeal[len(toSeal)-1], toSeal[i]
					toSeal = toSeal[:len(toSeal)-1]
					consecutiveFailures = 0
					break
				}
			}
			if len(toSeal) > 0 {
				consecutiveFailures++
				if consecutiveFailures == maxFailures {
					return fmt.Errorf("timed out waiting for leader election")
				}
				time.Sleep(attemptInterval)
			}
		}

		return nil
	})

	r.Dispatch("rekey", &app.Help{
		Summary: "Re-key your Vault with new unseal keys",
		Usage:   "vault-manager rekey [--gpg email@address ...] [--keys #] [--threshold #]",
		Type:    app.DestructiveCommand,
		Description: `
Rekeys Vault with new unseal keys. This will require a quorum
of existing unseal keys to accomplish. This command can be used
to change the nubmer of unseal keys being generated via --keys,
as well as the number of keys required to unseal the Vault via
--threshold.

If --gpg flags are provided, they will be used to look up in the
local GPG keyring public keys to give Vault for encrypting the new
unseal keys (one pubkey per unseal key). Output will have the
encrypted unseal keys, matched up with the email address associated
with the public key that it was encrypted with. Additionally, a
backup of the encrypted unseal keys is located at sys/rekey/backup
in Vault.

If no --gpg flags are provided, the output will include the raw
unseal keys, and should be treated accordingly.

By default, the new seal keys will also be stored in the Vault itself,
unless you specify the --no-persist flag.  They will be written to
secret/vault/seal/keys, as key1, key2, ... keyN.
`,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)

		unsealKeys := 5 // default to 5
		var gpgKeys []string
		if len(opt.Rekey.GPG) > 0 {
			unsealKeys = len(opt.Rekey.GPG)
			for _, email := range opt.Rekey.GPG {
				output, err := exec.Command("gpg", "--export", email).Output()
				if err != nil {
					return fmt.Errorf("Failed to retrieve GPG key for %s from local keyring: %w", email, err)
				}

				// gpg --export returns 0, with no stdout if the key wasn't found, so handle that
				if output == nil || len(output) == 0 {
					return fmt.Errorf("No GPG key found for %s in the local keyring", email)
				}
				gpgKeys = append(gpgKeys, base64.StdEncoding.EncodeToString(output))
			}
		}

		// if specified, --unseal-keys takes priority, then the number of --gpg-keys, and a default of 5
		if opt.Rekey.NKeys != 0 {
			unsealKeys = opt.Rekey.NKeys
		}
		if len(opt.Rekey.GPG) > 0 && unsealKeys != len(opt.Rekey.GPG) {
			return fmt.Errorf("Both --gpg and --keys were specified, and their counts did not match.")
		}

		// if --threshold isn't specified, use a default (unless default is > the number of keys
		if opt.Rekey.Threshold == 0 {
			opt.Rekey.Threshold = 3
			if opt.Rekey.Threshold > unsealKeys {
				opt.Rekey.Threshold = unsealKeys
			}
		}
		if opt.Rekey.Threshold > unsealKeys {
			return fmt.Errorf("You specified only %d unseal keys, but are requiring %d keys to unseal vault. This is bad.", unsealKeys, opt.Rekey.Threshold)
		}
		if opt.Rekey.Threshold < 2 && unsealKeys > 1 {
			return fmt.Errorf("When specifying more than 1 unseal key, you must also have more than one key required to unseal.")
		}

		v := app.Connect(true)
		keys, err := v.ReKey(unsealKeys, opt.Rekey.Threshold, gpgKeys)
		if err != nil {
			return err
		}

		if opt.Rekey.Persist {
			v.SaveSealKeys(keys)
		}

		fmt.Printf("@G{Your Vault has been re-keyed.} Please take note of your new unseal keys and @R{store them safely!}\n")
		for i, key := range keys {
			if len(opt.Rekey.GPG) == len(keys) {
				fmt.Printf("Unseal key for @c{%s}:\n@y{%s}\n", opt.Rekey.GPG[i], key)
			} else {
				fmt.Printf("Unseal key %d: @y{%s}\n", i+1, key)
			}
		}

		return nil
	})
}
