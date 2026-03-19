package cmd

import (
	"encoding/json"
	"encoding/pem"
	"crypto/x509"

	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/SomeBlackMagic/vault-manager/app"
	fmt "github.com/jhunt/go-ansi"
	"github.com/SomeBlackMagic/vault-manager/prompt"
	"github.com/SomeBlackMagic/vault-manager/rc"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

func registerTargetCommands(r *app.Runner, opt *Options) {
	r.Dispatch("targets", &app.Help{
		Summary: "List all targeted Vaults",
		Usage:   "vault-manager targets",
		Type:    app.AdministrativeCommand,
	}, func(command string, args ...string) error {
		if len(args) != 0 {
			r.ExitWithUsage("targets")
		}

		if opt.UseTarget != "" {
			fmt.Fprintf(os.Stderr, "@Y{Specifying --target to the targets command makes no sense; ignoring...}\n")
		}

		cfg := rc.Apply(opt.UseTarget)
		if opt.Targets.JSON {
			type vault struct {
				Name      string `json:"name"`
				URL       string `json:"url"`
				Verify    bool   `json:"verify"`
				Namespace string `json:"namespace,omitempty"`
				Strongbox bool   `json:"strongbox"`
			}
			vaults := make([]vault, 0)

			for name, details := range cfg.Vaults {
				vaults = append(vaults, vault{
					Name:      name,
					URL:       details.URL,
					Verify:    !details.SkipVerify,
					Namespace: details.Namespace,
					Strongbox: !details.NoStrongbox,
				})
			}
			b, err := json.MarshalIndent(vaults, "", "  ")
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", string(b))
			return nil
		}

		wide := 0
		keys := make([]string, 0)
		for name := range cfg.Vaults {
			keys = append(keys, name)
			if len(name) > wide {
				wide = len(name)
			}
		}

		currentFmt := fmt.Sprintf("(*) @G{%%-%ds}\t@R{%%s} @Y{%%s}\n", wide)
		otherFmt := fmt.Sprintf("    %%-%ds\t@R{%%s} %%s\n", wide)
		hasCurrent := ""
		if cfg.Current != "" {
			hasCurrent = " - current target indicated with a (*)"
		}

		fmt.Fprintf(os.Stderr, "\nKnown Vault targets%s:\n", hasCurrent)
		sort.Strings(keys)
		for _, name := range keys {
			t := cfg.Vaults[name]
			skip := "           "
			if t.SkipVerify {
				skip = " (noverify)"
			} else if strings.HasPrefix(t.URL, "http:") {
				skip = " (insecure)"
			}
			format := otherFmt
			if name == cfg.Current {
				format = currentFmt
			}
			fmt.Fprintf(os.Stderr, format, name, skip, t.URL)
		}
		fmt.Fprintf(os.Stderr, "\n")
		return nil
	})

	r.Dispatch("target", &app.Help{
		Summary: "Target a new Vault, or set your current Vault target",
		Description: `Target a new Vault if URL and ALIAS are provided, or set
your current Vault target if just ALIAS is given. If the single argument form
if provided, the following flags are valid:

-k (--insecure) specifies to skip x509 certificate validation. This only has an
effect if the given URL uses an HTTPS scheme.

-s (--strongbox) specifies that the targeted Vault has a strongbox deployed at
its IP on port :8484. This is true by default. --no-strongbox will cause commands
that would otherwise use strongbox to run against only the targeted Vault.

-n (--namespace) specifies a Vault Enterprise namespace to run commands against
for this target.

--ca-cert can be either a PEM-encoded certificate value or filepath to a
PEM-encoded certificate. The given certificate will be trusted as the signing
certificate to the certificate served by the Vault server. This flag can be
provided multiple times to provide multiple CA certificates.
`,
		Usage: "vault-manager [-k] [--[no]-strongbox] [-n] [--ca-cert] target [URL] [ALIAS] | vault-manager target -i",
		Type:  app.AdministrativeCommand,
	}, func(command string, args ...string) error {
		var cfg rc.Config
		if !opt.Target.Interactive && len(args) == 0 {
			cfg = rc.Apply(opt.UseTarget)
		} else {
			cfg = rc.Read()
		}
		skipverify := false
		if os.Getenv("VAULT_MANAGER_SKIP_VERIFY") == "1" {
			skipverify = true
		}

		if opt.UseTarget != "" {
			fmt.Fprintf(os.Stderr, "@Y{Specifying --target to the target command makes no sense; ignoring...}\n")
		}

		printTarget := func() {
			u := cfg.URL()
			fmt.Fprintf(os.Stderr, "Currently targeting @C{%s} at @C{%s}\n", cfg.Current, u)
			if !cfg.Verified() {
				fmt.Fprintf(os.Stderr, "@R{Skipping TLS certificate validation}\n")
			}
			if cfg.Namespace() != "" {
				fmt.Fprintf(os.Stderr, "Using namespace @C{%s}\n", cfg.Namespace())
			}
			if cfg.HasStrongbox() {
				urlAsURL, err := url.Parse(u)
				fmt.Fprintf(os.Stderr, "Uses Strongbox")
				if err == nil {
					fmt.Fprintf(os.Stderr, " at @C{%s}", vault.StrongboxURL(urlAsURL))
				}
				fmt.Fprintf(os.Stderr, "\n")
			} else {
				fmt.Fprintf(os.Stderr, "Does not use Strongbox\n")
			}
			fmt.Fprintf(os.Stderr, "\n")
		}

		if opt.Target.Interactive {
			for {
				if len(cfg.Vaults) == 0 {
					fmt.Fprintf(os.Stderr, "@R{No Vaults have been targeted yet.}\n\n")
					fmt.Fprintf(os.Stderr, "You will need to target a Vault manually first.\n\n")
					fmt.Fprintf(os.Stderr, "Try something like this:\n")
					fmt.Fprintf(os.Stderr, "     @C{vault-manager target ops https://address.of.your.vault}\n")
					fmt.Fprintf(os.Stderr, "     @C{vault-manager auth (github|token|ldap|okta|userpass)}\n")
					fmt.Fprintf(os.Stderr, "\n")
					os.Exit(1)
				}
				r.Execute("targets")

				fmt.Fprintf(os.Stderr, "Which Vault would you like to target?\n")
				t := prompt.Normal("@G{> }")
				err := cfg.SetCurrent(t, skipverify)
				if err != nil {
					fmt.Fprintf(os.Stderr, "@R{%s}\n", err)
					continue
				}
				err = cfg.Write()
				if err != nil {
					return err
				}
				if !opt.Quiet {
					skip := ""
					if !cfg.Verified() {
						skip = " (skipping TLS certificate verification)"
					}
					fmt.Fprintf(os.Stderr, "Now targeting @C{%s} at @C{%s}@R{%s}\n\n", cfg.Current, cfg.URL(), skip)
				}
				return nil
			}
		}
		if len(args) == 0 {
			if !opt.Quiet {
				if opt.Target.JSON {
					var out struct {
						Name      string `json:"name"`
						URL       string `json:"url"`
						Verify    bool   `json:"verify"`
						Strongbox bool   `json:"strongbox"`
					}
					if cfg.Current != "" {
						out.Name = cfg.Current
						out.URL = cfg.URL()
						out.Verify = cfg.Verified()
						out.Strongbox = cfg.HasStrongbox()
					}
					b, err := json.MarshalIndent(&out, "", "  ")
					if err != nil {
						return err
					}
					fmt.Printf("%s\n", string(b))
					return nil
				}

				if cfg.Current == "" {
					fmt.Fprintf(os.Stderr, "@R{No Vault currently targeted}\n")
				} else {
					printTarget()
				}
			}
			return nil
		}
		if len(args) == 1 {
			err := cfg.SetCurrent(args[0], skipverify)
			if err != nil {
				return err
			}
			if !opt.Quiet {
				printTarget()
			}
			return cfg.Write()
		}

		if len(args) == 2 {
			var err error
			alias, url := args[0], args[1]
			if !(strings.HasPrefix(args[1], "http://") ||
				strings.HasPrefix(args[1], "https://")) {
				alias, url = url, alias
			}

			caCerts := []string{}
			for _, input := range opt.Target.CACerts {
				const errorPrefix = "Error reading CA certificates"
				p, _ := pem.Decode([]byte(input))
				// If not a PEM block, try to interpret it as a filepath pointing to
				// a file that contains a PEM block.
				if p == nil {
					pemData, err := os.ReadFile(input)
					if err != nil {
						return fmt.Errorf("%s: While reading from file `%s': %w", errorPrefix, input, err)
					}

					p, _ = pem.Decode([]byte(pemData))
					if p == nil {
						return fmt.Errorf("%s: File contents could not be parsed as PEM-encoded data", errorPrefix)
					}
				}

				_, err := x509.ParseCertificate(p.Bytes)
				if err != nil {
					return fmt.Errorf("%s: While parsing certificate ASN.1 DER data: %w", errorPrefix, err)
				}

				toWrite := pem.EncodeToMemory(p)
				caCerts = append(caCerts, string(toWrite))
			}

			err = cfg.SetTarget(alias, rc.Vault{
				URL:         url,
				SkipVerify:  skipverify,
				NoStrongbox: !opt.Target.Strongbox,
				Namespace:   opt.Target.Namespace,
				CACerts:     caCerts,
			})
			if err != nil {
				return err
			}
			if !opt.Quiet {
				printTarget()
			}
			return cfg.Write()
		}

		r.ExitWithUsage("target")
		return nil
	})

	r.Dispatch("target delete", &app.Help{
		Summary: "Forget about a targeted Vault",
		Usage:   "vault-manager target delete ALIAS",
		Type:    app.DestructiveCommand,
	}, func(command string, args ...string) error {
		cfg := rc.Apply(opt.UseTarget)
		if len(args) != 1 {
			r.ExitWithUsage("target delete")
		}

		delete(cfg.Vaults, args[0])
		if cfg.Current == args[0] {
			cfg.Current = ""
		}

		return cfg.Write()
	})

	r.Dispatch("env", &app.Help{
		Summary: "Print the environment variables for the current target",
		Usage:   "vault-manager env",
		Description: `
Print the environment variables representing the current target.

 --bash   Format the environment variables to be used by Bash.

 --fish   Format the environment variables to be used by fish.

 --json   Format the environment variables in json format.

Please note that if you specify --json, --bash or --fish then the output will be
written to STDOUT instead of STDERR to make it easier to consume.
		`,
		Type: app.AdministrativeCommand,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		if opt.Env.ForBash && opt.Env.ForFish && opt.Env.ForJSON {
			r.Help(os.Stderr, "env")
			fmt.Fprintf(os.Stderr, "@R{Only specify one of --json, --bash OR --fish.}\n")
			os.Exit(1)
		}
		vars := map[string]string{
			"VAULT_ADDR":        os.Getenv("VAULT_ADDR"),
			"VAULT_TOKEN":       os.Getenv("VAULT_TOKEN"),
			"VAULT_SKIP_VERIFY": os.Getenv("VAULT_SKIP_VERIFY"),
			"VAULT_NAMESPACE":   os.Getenv("VAULT_NAMESPACE"),
		}

		switch {
		case opt.Env.ForBash:
			for name, value := range vars {
				if value != "" {
					fmt.Fprintf(os.Stdout, "\\export %s=%s;\n", name, value)
				} else {
					fmt.Fprintf(os.Stdout, "\\unset %s;\n", name)
				}
			}
		case opt.Env.ForFish:
			for name, value := range vars {
				if value == "" {
					fmt.Fprintf(os.Stdout, "set -u %s;\n", name)
				} else {
					fmt.Fprintf(os.Stdout, "set -x %s %s;\n", name, value)
				}
			}
		case opt.Env.ForJSON:
			jsonEnv := &struct {
				Addr  string `json:"VAULT_ADDR"`
				Token string `json:"VAULT_TOKEN,omitempty"`
				Skip  string `json:"VAULT_SKIP_VERIFY,omitempty"`
				NS    string `json:"VAULT_NAMESPACE,omitempty"`
			}{
				Addr:  vars["VAULT_ADDR"],
				Token: vars["VAULT_TOKEN"],
				Skip:  vars["VAULT_SKIP_VERIFY"],
				NS:    vars["VAULT_NAMESPACE"],
			}
			b, err := json.Marshal(jsonEnv)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", string(b))
			return nil

		default:
			for name, value := range vars {
				if value != "" {
					fmt.Fprintf(os.Stderr, "  @B{%s}  @G{%s}\n", name, value)
				}
			}
		}
		return nil
	})
}
