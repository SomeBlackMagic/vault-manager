package cmd

import (
	"os"
	"strconv"

	"github.com/SomeBlackMagic/vault-manager/app"
	fmt "github.com/jhunt/go-ansi"
	"github.com/SomeBlackMagic/vault-manager/rc"
	"github.com/SomeBlackMagic/vault-manager/vault"

	uuid "github.com/pborman/uuid"
)

func registerGenerateCommands(r *app.Runner, opt *Options) {
	r.Dispatch("gen", &app.Help{
		Summary: "Generate a random password",
		Usage:   "vault-manager gen [-l <length>] [-p] PATH:KEY [PATH:KEY ...]",
		Type:    app.DestructiveCommand,
		Description: `
LENGTH defaults to 64 characters.

The following options are recognized:

  -l, --length  Specify the length of the random string to generate
	-p, --policy  Specify a regex character grouping for limiting characters used
	              to generate the password (e.g --policy a-z0-9)
`,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)

		if len(args) == 0 {
			r.ExitWithUsage("gen")
		}

		length := 64

		if opt.Gen.Length != 0 {
			length = opt.Gen.Length
		} else if u, err := strconv.ParseUint(args[0], 10, 16); err == nil {
			length = int(u)
			args = args[1:]
		}

		v := app.Connect(true)

		for len(args) > 0 {
			var path, key string
			if vault.PathHasKey(args[0]) {
				path, key, _ = vault.ParsePath(args[0])
				args = args[1:]
			} else {
				if len(args) < 2 {
					r.ExitWithUsage("gen")
				}
				path, key = args[0], args[1]
				//If the key looks like a full path with a :key at the end, then the user
				// probably botched the args
				if vault.PathHasKey(key) {
					return fmt.Errorf("For secret `%s` and key `%s`: key cannot contain a key", path, key)
				}
				args = args[2:]
			}
			s, err := v.Read(path)
			if err != nil && !vault.IsNotFound(err) {
				return err
			}
			exists := (err == nil)
			if opt.SkipIfExists && exists && s.Has(key) {
				if !opt.Quiet {
					fmt.Fprintf(os.Stderr, "@R{Cowardly refusing to update} @C{%s:%s} @R{as it is already present in Vault}\n", path, key)
				}
				continue
			}
			err = s.Password(key, length, opt.Gen.Policy, opt.SkipIfExists)
			if err != nil {
				return err
			}

			if err = v.Write(path, s); err != nil {
				return err
			}
		}
		return nil
	})

	r.Dispatch("uuid", &app.Help{
		Summary:     "Generate a new UUIDv4",
		Usage:       "vault-manager uuid PATH[:KEY]",
		Type:        app.DestructiveCommand,
		Description: ``,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)

		if len(args) != 1 {
			r.ExitWithUsage("uuid")
		}

		u := uuid.NewRandom()

		stringuuid := u.String()

		v := app.Connect(true)

		var path, key string
		if vault.PathHasKey(args[0]) {
			path, key, _ = vault.ParsePath(args[0])

		} else {
			path, key = args[0], "uuid"
			//If the key looks like a full path with a :key at the end, then the user
			//probably botched the args
			if vault.PathHasKey(key) {
				return fmt.Errorf("For secret `%s` and key `%s`: key cannot contain a key", path, key)
			}

		}
		s, err := v.Read(path)
		if err != nil && !vault.IsNotFound(err) {
			return err
		}
		exists := (err == nil)
		if opt.SkipIfExists && exists && s.Has(key) {
			if !opt.Quiet {
				fmt.Fprintf(os.Stderr, "@R{Cowardly refusing to update} @C{%s:%s} @R{as it is already present in Vault}\n", path, key)
			}
			return err
		}
		err = s.Set(key, stringuuid, opt.SkipIfExists)
		if err != nil {
			return err
		}

		if err = v.Write(path, s); err != nil {
			return err
		}

		return nil
	})

	r.Dispatch("ssh", &app.Help{
		Summary: "Generate one or more new SSH RSA keypair(s)",
		Usage:   "vault-manager ssh [NBITS] PATH [PATH ...]",
		Type:    app.DestructiveCommand,
		Description: `
For each PATH given, a new SSH RSA public/private keypair will be generated,
with a key strength of NBITS (which defaults to 2048).  The private keys will
be stored under the 'private' name, as a PEM-encoded RSA private key, and the
public key, formatted for use in an SSH authorized_keys file, under 'public'.
`,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		bits := 2048
		if len(args) > 0 {
			if u, err := strconv.ParseUint(args[0], 10, 16); err == nil {
				bits = int(u)
				args = args[1:]
			}
		}

		if len(args) < 1 {
			r.ExitWithUsage("ssh")
		}

		v := app.Connect(true)
		for _, path := range args {
			s, err := v.Read(path)
			if err != nil && !vault.IsNotFound(err) {
				return err
			}
			exists := (err == nil)
			if opt.SkipIfExists && exists && (s.Has("private") || s.Has("public") || s.Has("fingerprint")) {
				if !opt.Quiet {
					fmt.Fprintf(os.Stderr, "@R{Cowardly refusing to generate an SSH key at} @C{%s} @R{as it is already present in Vault}\n", path)
				}
				continue
			}
			if err = s.SSHKey(bits, opt.SkipIfExists); err != nil {
				return err
			}
			if err = v.Write(path, s); err != nil {
				return err
			}
		}
		return nil
	})

	r.Dispatch("rsa", &app.Help{
		Summary: "Generate a new RSA keypair",
		Usage:   "vault-manager rsa [NBITS] PATH [PATH ...]",
		Type:    app.DestructiveCommand,
		Description: `
For each PATH given, a new RSA public/private keypair will be generated with a,
key strength of NBITS (which defaults to 2048).  The private keys will be stored
under the 'private' name, and the public key under the 'public' name.  Both will
be PEM-encoded.
`,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		bits := 2048
		if len(args) > 0 {
			if u, err := strconv.ParseUint(args[0], 10, 16); err == nil {
				bits = int(u)
				args = args[1:]
			}
		}

		if len(args) < 1 {
			r.ExitWithUsage("rsa")
		}

		v := app.Connect(true)
		for _, path := range args {
			s, err := v.Read(path)
			if err != nil && !vault.IsNotFound(err) {
				return err
			}
			exists := (err == nil)
			if opt.SkipIfExists && exists && (s.Has("private") || s.Has("public")) {
				if !opt.Quiet {
					fmt.Fprintf(os.Stderr, "@R{Cowardly refusing to generate an RSA key at} @C{%s} @R{as it is already present in Vault}\n", path)
				}
				continue
			}
			if err = s.RSAKey(bits, opt.SkipIfExists); err != nil {
				return err
			}
			if err = v.Write(path, s); err != nil {
				return err
			}
		}
		return nil
	})

	r.Dispatch("dhparam", &app.Help{
		Summary: "Generate Diffie-Helman key exchange parameters",
		Usage:   "vault-manager dhparam [NBITS] PATH",
		Type:    app.DestructiveCommand,
		Description: `
NBITS defaults to 2048.
`,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		bits := 2048

		if len(args) > 0 {
			if u, err := strconv.ParseUint(args[0], 10, 16); err == nil {
				bits = int(u)
				args = args[1:]
			}
		}

		if len(args) < 1 {
			r.ExitWithUsage("dhparam")
		}

		path := args[0]
		v := app.Connect(true)
		s, err := v.Read(path)
		if err != nil && !vault.IsNotFound(err) {
			return err
		}
		exists := (err == nil)
		if opt.SkipIfExists && exists && s.Has("dhparam-pem") {
			if !opt.Quiet {
				fmt.Fprintf(os.Stderr, "@R{Cowardly refusing to generate a Diffie-Hellman key exchange parameter set at} @C{%s} @R{as it is already present in Vault}\n", path)
			}
			return nil
		}
		if err = s.DHParam(bits, opt.SkipIfExists); err != nil {
			return err
		}
		return v.Write(path, s)
	})
}
