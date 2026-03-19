package cmd

import (
	"encoding/json"
	"os"

	"github.com/cloudfoundry-community/vaultkv"
	fmt "github.com/jhunt/go-ansi"
	"github.com/SomeBlackMagic/vault-manager/app"
	"github.com/SomeBlackMagic/vault-manager/prompt"
	"github.com/SomeBlackMagic/vault-manager/rc"
)

func registerAuthCommands(r *app.Runner, opt *Options) {
	r.Dispatch("auth", &app.Help{
		Summary: "Authenticate to the current target",
		Usage:   "vault-manager auth [--path <value>] (token|github|ldap|okta|userpass|approle)",
		Description: `
Set the authentication token sent when talking to the Vault.

Supported auth backends are:

token     Set the Vault authentication token directly.
github    Provide a Github personal access (oauth) token.
ldap      Provide LDAP user credentials.
okta      Provide Okta user credentials.
userpass  Provide a username and password registered with the UserPass backend.
approle   Provide a client ID and client secret registered with the AppRole backend.
status    Get information about current authentication status

Flags:
  -p, --path  Set the path of the auth backend mountpoint. For those who are
              familiar with the API, this is the part that comes after v1/auth.
              Defaults to the name of auth type (e.g. "userpass"), which is
              the default when creating auth backends with the Vault CLI.
  -j, --json  For auth status, returns the information as a JSON object.
`,
		Type: app.AdministrativeCommand,
	}, func(command string, args ...string) error {
		cfg := rc.Apply(opt.UseTarget)
		v := app.Connect(false)
		v.Client().Client.SetAuthToken("")

		method := "token"
		if len(args) > 0 {
			method = args[0]
			args = args[1:]
		}

		var token string
		var err error
		url := os.Getenv("VAULT_ADDR")
		target := cfg.Current
		if opt.UseTarget != "" {
			target = opt.UseTarget
		}
		fmt.Fprintf(os.Stderr, "Authenticating against @C{%s} at @C{%s}\n", target, url)

		authMount := method
		if opt.Auth.Path != "" {
			authMount = opt.Auth.Path
		}

		switch method {
		case "token":
			if opt.Auth.Path != "" {
				return fmt.Errorf("Setting a custom path is not supported for token auth")
			}
			token = prompt.Secure("Token: ")

		case "ldap":
			username := prompt.Normal("LDAP username: ")
			password := prompt.Secure("Password: ")

			result, err := v.Client().Client.AuthLDAPMount(authMount, username, password)
			if err != nil {
				return err
			}
			token = result.ClientToken

		case "okta":
			username := prompt.Normal("Okta username: ")
			password := prompt.Secure("Password: ")

			result, err := v.Client().Client.AuthOktaMount(authMount, username, password)
			if err != nil {
				return err
			}
			token = result.ClientToken

		case "github":
			accessToken := prompt.Secure("Github Personal Access Token: ")

			result, err := v.Client().Client.AuthGithubMount(authMount, accessToken)
			if err != nil {
				return err
			}
			token = result.ClientToken

		case "userpass":
			username := prompt.Normal("Username: ")
			password := prompt.Secure("Password: ")

			result, err := v.Client().Client.AuthUserpassMount(authMount, username, password)
			if err != nil {
				return err
			}
			token = result.ClientToken

		case "approle":
			roleID := prompt.Normal("Role ID: ")
			secretID := prompt.Secure("Secret ID: ")

			result, err := v.Client().Client.AuthApproleMount(authMount, roleID, secretID)
			if err != nil {
				return err
			}
			token = result.ClientToken

		case "status":
			v := app.Connect(false)
			tokenInfo, err := v.Client().Client.TokenInfoSelf()
			var tokenObj app.TokenStatus
			if err != nil {
				if !(vaultkv.IsForbidden(err) ||
					vaultkv.IsNotFound(err) ||
					vaultkv.IsBadRequest(err)) {
					return err
				}
			} else {
				tokenObj.Info = *tokenInfo
				tokenObj.Valid = true
			}

			var output string
			if opt.Auth.JSON {
				outputBytes, err := json.MarshalIndent(tokenObj, "", "  ")
				if err != nil {
					panic("Could not marshal json from TokenStatus object")
				}

				output = string(append(outputBytes, '\n'))
			} else {
				output = tokenObj.String()
			}

			fmt.Printf(output)
			return nil

		default:
			return fmt.Errorf("Unrecognized authentication method '%s'", method)
		}

		//This handles saving the token to the correct target when using the -T
		// flag to use a different target
		currentTarget := cfg.Current
		err = cfg.SetCurrent(target, false)
		if err != nil {
			return fmt.Errorf("Could not find target with name `%s'")
		}
		cfg.SetToken(token)
		cfg.SetCurrent(currentTarget, false)
		return cfg.Write()
	})

	r.Dispatch("logout", &app.Help{
		Summary: "Forget the authentication token of the currently targeted Vault",
		Usage:   "vault-manager logout\n",
		Type:    app.AdministrativeCommand,
	}, func(command string, args ...string) error {
		cfg := rc.Apply(opt.UseTarget)
		cfg.SetToken("")
		err := cfg.Write()
		if err != nil {
			return err
		}

		target := cfg.Current
		if opt.UseTarget != "" {
			target = opt.UseTarget
		}
		fmt.Fprintf(os.Stderr, "Successfully logged out of @C{%s}\n", target)
		return nil
	})

	r.Dispatch("renew", &app.Help{
		Summary: "Renew one or more authentication tokens",
		Usage:   "vault-manager renew [all]\n",
		Type:    app.AdministrativeCommand,
	}, func(command string, args ...string) error {
		if len(args) > 0 {
			if len(args) != 1 || args[0] != "all" {
				r.ExitWithUsage("renew")
			}
			cfg := rc.Apply("")
			failed := 0
			for vault := range cfg.Vaults {
				rc.Apply(vault)
				if os.Getenv("VAULT_TOKEN") == "" {
					fmt.Printf("skipping @C{%s} - no token found.\n", vault)
					continue
				}
				fmt.Printf("renewing token against @C{%s}...\n", vault)
				v := app.Connect(true)
				if err := v.RenewLease(); err != nil {
					fmt.Fprintf(os.Stderr, "@R{failed to renew token against %s: %s}\n", vault, err)
					failed++
				}
			}
			if failed > 0 {
				return fmt.Errorf("failed to renew %d token(s)", failed)
			}
			return nil
		}

		rc.Apply(opt.UseTarget)
		v := app.Connect(true)
		if err := v.RenewLease(); err != nil {
			return err
		}
		return nil
	})
}
