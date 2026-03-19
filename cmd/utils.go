package cmd

import (
	"io"
	"net/http/httputil"
	"os"
	"os/exec"
	"strings"

	"github.com/jhunt/go-ansi"
	fmt "github.com/jhunt/go-ansi"
	"github.com/SomeBlackMagic/vault-manager/app"
	"github.com/SomeBlackMagic/vault-manager/rc"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

func registerUtilsCommands(r *app.Runner, opt *Options) {
	r.Dispatch("fmt", &app.Help{
		Summary: "Reformat an existing name/value pair, into a new name",
		Usage:   "vault-manager fmt FORMAT PATH OLD-NAME NEW-NAME",
		Type:    app.DestructiveCommand,
		Description: `
Take the value stored at PATH/OLD-NAME, format it a different way, and
then save it at PATH/NEW-NAME.  This can be useful for generating a new
password (via 'vault-manager gen') and then crypt'ing it for use in /etc/shadow,
using the 'crypt-sha512' format.

Supported formats:

    base64          Base64 encodes the value
    bcrypt          Salt and hash the value, using bcrypt (Blowfish, in crypt format).
    crypt-md5       Salt and hash the value, using MD5, in crypt format (legacy).
    crypt-sha256    Salt and hash the value, using SHA-256, in crypt format.
    crypt-sha512    Salt and hash the value, using SHA-512, in crypt format.

`,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)

		if len(args) != 4 {
			r.ExitWithUsage("fmt")
		}

		fmtType := args[0]
		path := args[1]
		oldKey := args[2]
		newKey := args[3]

		v := app.Connect(true)
		s, err := v.Read(path)
		if err != nil {
			return err
		}
		if opt.SkipIfExists && s.Has(newKey) {
			if !opt.Quiet {
				fmt.Fprintf(os.Stderr, "@R{Cowardly refusing to reformat} @C{%s:%s} @R{to} @C{%s} @R{as it is already present in Vault}\n", path, oldKey, newKey)
			}
			return nil
		}
		if err = s.Format(oldKey, newKey, fmtType, opt.SkipIfExists); err != nil {
			if vault.IsNotFound(err) {
				return fmt.Errorf("%s:%s does not exist, cannot create %s encoded copy at %s:%s", path, oldKey, fmtType, path, newKey)
			}
			return fmt.Errorf("Error encoding %s:%s as %s: %w", path, oldKey, fmtType, err)
		}

		return v.Write(path, s)
	})

	r.Dispatch("prompt", &app.Help{
		Summary: "Print a prompt (useful for scripting vault-manager command sets)",
		Usage:   "vault-manager echo Your Message Here:",
		Type:    app.NonDestructiveCommand,
	}, func(command string, args ...string) error {
		// --no-clobber is ignored here, because there's no context of what you're
		// about to be writing after a prompt, so not sure if we should or shouldn't prompt
		// if you need to write something and prompt, but only if it isnt already present
		// in vault, see the `ask` subcommand
		fmt.Fprintf(os.Stderr, "%s\n", strings.Join(args, " "))
		return nil
	})

	r.Dispatch("option", &app.Help{
		Summary: "View or edit global vault-manager CLI options",
		Usage:   "vault-manager option [optionname=value]",
		Type:    app.AdministrativeCommand,
		Description: `
Currently available options are:

@G{manage_vault_token}    If set to true, then when logging in or switching targets,
                      the '.vault-token' file in your $HOME directory that the Vault CLI uses will be
                      updated.
`,
	}, func(command string, args ...string) error {
		cfg := rc.Apply(opt.UseTarget)

		optLookup := []struct {
			opt string
			val *bool
		}{
			{"manage_vault_token", &cfg.Options.ManageVaultToken},
		}

		if len(args) == 0 {
			tbl := app.Table{}
			for _, entry := range optLookup {
				value := "@R{false}"
				if *entry.val {
					value = "@G{true}"
				}
				tbl.AddRow(entry.opt, ansi.Sprintf(value))
			}

			tbl.Print()
			return nil
		}

		for _, arg := range args {
			argSplit := strings.Split(arg, "=")
			if len(argSplit) != 2 {
				return fmt.Errorf("Option arg syntax: option=value")
			}

			parseTrueFalse := func(s string) (bool, error) {
				switch s {
				case "true", "on", "yes":
					return true, nil
				case "false", "off", "no":
					return false, nil
				}

				return false, fmt.Errorf("value must be one of true|on|yes|false|off|no")
			}

			optionKey := strings.ReplaceAll(argSplit[0], "-", "_")
			optionVal, err := parseTrueFalse(argSplit[1])
			if err != nil {
				return err
			}

			found := false
			for _, o := range optLookup {
				if o.opt == optionKey {
					found = true
					*o.val = optionVal
					ansi.Printf("updated @G{%s}\n", o.opt)
					break
				}
			}

			if !found {
				return fmt.Errorf("unknown option: %s", argSplit[0])
			}
		}

		return cfg.Write()
	})

	r.Dispatch("vault", &app.Help{
		Summary: "Run arbitrary Vault CLI commands against the current target",
		Usage:   "vault-manager vault ...",
		Type:    app.DestructiveCommand,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)

		if opt.SkipIfExists {
			fmt.Fprintf(os.Stderr, "@C{--no-clobber} @Y{specified, but is ignored for} @C{vault-manager vault}\n")
		}

		proxy, err := vault.NewProxyRouter()
		if err != nil {
			return err
		}

		cmd := exec.Command("vault", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		//If the command is vault status, we don't want to expose the VAULT_NAMESPACE envvar
		for _, arg := range args {
			if !strings.HasPrefix(arg, "-") {
				if arg == "status" {
					os.Unsetenv("VAULT_NAMESPACE")
				}
				break
			}
		}
		cmd.Env = os.Environ()

		//Make sure we don't accidentally specify a http_proxy and a HTTP_PROXY
		for i := range cmd.Env {
			parts := strings.Split(cmd.Env[i], "=")
			if len(parts) < 2 {
				continue
			}
			if parts[0] == "http_proxy" || parts[0] == "https_proxy" || parts[0] == "no_proxy" {
				cmd.Env[i] = strings.ToUpper(parts[0]) + "=" + strings.Join(parts[1:], "=")
			}
		}

		if proxy.ProxyConf.HTTPProxy != "" {
			cmd.Env = append(cmd.Env, "HTTP_PROXY="+proxy.ProxyConf.HTTPProxy)
		}

		if proxy.ProxyConf.HTTPSProxy != "" {
			cmd.Env = append(cmd.Env, "HTTPS_PROXY="+proxy.ProxyConf.HTTPSProxy)
		}

		if proxy.ProxyConf.NoProxy != "" {
			cmd.Env = append(cmd.Env, "NO_PROXY="+proxy.ProxyConf.NoProxy)
		}

		err = cmd.Run()
		if err != nil {
			return err
		}
		return nil
	})

	r.Dispatch("curl", &app.Help{
		Summary: "Issue arbitrary HTTP requests to the current Vault (for diagnostics)",
		Usage:   "vault-manager curl [OPTIONS] METHOD REL-URI [DATA]",
		Type:    app.DestructiveCommand,
		Description: `
This is a debugging and diagnostics tool.  You should not need to use
'vault-manager curl' for normal operation or interaction with a Vault.

The following OPTIONS are recognized:

  --data-only         Show only the response body, hiding the response headers.

METHOD must be one of GET, LIST, POST, or PUT.

REL-URI is the relative URI (the path component, starting with the first
forward slash) of the resource you wish to access.

DATA should be a JSON string, since almost all of the Vault API handlers
deal exclusively in JSON payloads.  GET requests should not have DATA.
Query string parameters should be appended to REL-URI, instead of being
sent as DATA.
`,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)

		var (
			url, method string
			data        []byte
		)

		method = "GET"
		if len(args) < 1 {
			r.ExitWithUsage("curl")
		} else if len(args) == 1 {
			url = args[0]
		} else {
			method = strings.ToUpper(args[0])
			url = args[1]
			data = []byte(strings.Join(args[2:], " "))
		}

		v := app.Connect(true)
		res, err := v.Curl(method, url, data)
		if err != nil {
			return err
		}

		if opt.Curl.DataOnly {
			b, err := io.ReadAll(res.Body)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "%s\n", string(b))

		} else {
			r, _ := httputil.DumpResponse(res, true)
			fmt.Fprintf(os.Stdout, "%s\n", r)
		}
		return nil
	})
}
