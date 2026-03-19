package main

import (
	"errors"
	"os"

	fmt "github.com/jhunt/go-ansi"
	"github.com/jhunt/go-cli"
	env "github.com/jhunt/go-envirotron"
	"github.com/SomeBlackMagic/vault-manager/app"
	"github.com/SomeBlackMagic/vault-manager/cmd"
	"github.com/SomeBlackMagic/vault-manager/rc"
)

// Version is set at build time via -ldflags "-X main.Version=...".
// This is the standard Go practice for CLI build-time variables and is exempt
// from the global variable prohibition.
var Version string

func main() {
	opt := cmd.NewOptions()

	go app.Signals()

	r := app.NewRunner()

	cmd.RegisterAll(r, opt, Version)

	env.Override(opt)
	p, err := cli.NewParser(opt, os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "@R{!! %s}\n", err)
		os.Exit(1)
	}

	if opt.Version {
		r.Execute("version")
		return
	}
	if opt.Help { //-h was given as a global arg
		r.Execute("help")
		return
	}

	for p.Next() {
		opt.SkipIfExists = !opt.Clobber

		if opt.Version {
			r.Execute("version")
			return
		}

		if p.Command == "" { //No recognized command was found
			r.Execute("help")
			return
		}

		if opt.Help { // -h or --help was given after a command
			r.Execute("help", p.Command)
			continue
		}

		os.Unsetenv("VAULT_SKIP_VERIFY")
		os.Unsetenv("VAULT_MANAGER_SKIP_VERIFY")
		if opt.Insecure {
			os.Setenv("VAULT_SKIP_VERIFY", "1")
			os.Setenv("VAULT_MANAGER_SKIP_VERIFY", "1")
		}

		defer rc.Cleanup()
		err = r.Execute(p.Command, p.Args...)
		if err != nil {
			var usageErr *app.UsageError
			if errors.As(err, &usageErr) {
				fmt.Fprintf(os.Stderr, "@Y{%s}\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "@R{!! %s}\n", err)
			}
			os.Exit(1)
		}
	}

	//If there were no args given, the above loop that would try to give help
	// doesn't execute at all, so we catch it here.
	if p.Command == "" {
		r.Execute("help")
	}

	if err = p.Error(); err != nil {
		fmt.Fprintf(os.Stderr, "@R{!! %s}\n", err)
		os.Exit(1)
	}
}
