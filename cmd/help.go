package cmd

import (
	"os"
	"strings"

	"github.com/SomeBlackMagic/vault-manager/app"
	fmt "github.com/jhunt/go-ansi"
)

func registerHelpCommands(r *app.Runner, opt *Options, version string, revision string) {
	r.Dispatch("version", &app.Help{
		Summary: "Print the version of the vault-manager CLI",
		Usage:   "vault-manager version",
		Type:    app.AdministrativeCommand,
	}, func(command string, args ...string) error {
		if version != "" {
			fmt.Fprintf(os.Stderr, "vault-manager v%s(%s)\n", version, revision)
		} else {
			fmt.Fprintf(os.Stderr, "vault-manager (development build)\n")
		}
		os.Exit(0)
		return nil
	})

	r.Dispatch("help", nil, func(command string, args ...string) error {
		if len(args) == 0 {
			args = append(args, "commands")
		}
		r.Help(os.Stderr, strings.Join(args, " "))
		os.Exit(0)
		return nil
	})

	r.Dispatch("envvars", nil, func(command string, args ...string) error {
		fmt.Printf(`@G{[SCRIPTING]}
  @B{VAULT_MANAGER_TARGET}    The vault alias which requests are sent to.

@G{[PROXYING]}
  @B{HTTP_PROXY}     The proxy to use for HTTP requests.
  @B{HTTPS_PROXY}    The proxy to use for HTTPS requests.
  @B{VAULT_MANAGER_ALL_PROXY} The proxy to use for both HTTP and HTTPS requests.
                 Overrides HTTP_PROXY and HTTPS_PROXY.
  @B{NO_PROXY}       A comma-separated list of domains to not use proxies for.
  @B{VAULT_MANAGER_KNOWN_HOSTS_FILE}
                 The location of your known hosts file, used for
                 'ssh+socks5://' proxying. Uses '${HOME}/.ssh/known_hosts'
                 by default.
  @B{VAULT_MANAGER_SKIP_HOST_KEY_VALIDATION}
                 If set, 'ssh+socks5://' proxying will skip host key validation
                 validation of the remote ssh server.


  The proxy environment variables support proxies with the schemes 'http://',
  'https://', 'socks5://', or 'ssh+socks5://'. http, https, and socks5 do what they
  say - they'll proxy through the server with the hostname:port given using the
  protocol specified in the scheme.

  'ssh+socks5://' will open an SSH tunnel to the given server, then will start a
  local SOCKS5 proxy temporarily which sends its traffic through the SSH tunnel.
  Because this requires an SSH connection, some extra information is required.
  This type of proxy should be specified in the form

      ssh+socks5://<user>@<hostname>:<port>/<path-to-private-key>
  or  ssh+socks5://<user>@<hostname>:<port>?private-key=<path-to-private-key

  If no port is provided, port 22 is assumed.
  Encrypted private keys are not supported. Password authentication is also not
  supported.

  Your known_hosts file is used to verify the remote ssh server's host key. If no
  key for the given server is present, you will be prompted to add the key. If no
  TTY when no host key is present, vault-manager will return with a failure.

`)
		return nil
	})
}
