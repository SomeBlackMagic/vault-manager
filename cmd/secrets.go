package cmd

import (
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/SomeBlackMagic/vault-manager/app"
	fmt "github.com/jhunt/go-ansi"
	"github.com/SomeBlackMagic/vault-manager/rc"
	"github.com/SomeBlackMagic/vault-manager/vault"
	"gopkg.in/yaml.v2"
)

func registerSecretCommands(r *app.Runner, opt *Options) {
	writeHelper := func(prompt bool, insecure bool, command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		if len(args) < 2 {
			r.ExitWithUsage(command)
		}
		v := app.Connect(true)
		path, args := args[0], args[1:]
		s, err := v.Read(path)
		if err != nil && !vault.IsNotFound(err) {
			return err
		}
		exists := (err == nil)
		clobberKeys := []string{}
		for _, arg := range args {
			k, val, missing, err := app.ParseKeyVal(arg, opt.Quiet)
			if err != nil {
				return err
			}
			if opt.SkipIfExists && exists && s.Has(k) {
				clobberKeys = append(clobberKeys, k)
				continue
			}
			// realize that we're going to fail, and don't prompt the user for any info
			if len(clobberKeys) > 0 {
				continue
			}
			if missing {
				val = app.Pr(k, prompt, insecure)
			}
			if err != nil {
				return err
			}
			err = s.Set(k, val, opt.SkipIfExists)
			if err != nil {
				return err
			}
		}
		if len(clobberKeys) > 0 {
			if !opt.Quiet {
				fmt.Fprintf(os.Stderr, "@R{Cowardly refusing to update} @C{%s}@R{, as the following keys would be clobbered:} @C{%s}\n",
					path, strings.Join(clobberKeys, ", "))
			}
			return nil
		}
		return v.Write(path, s)
	}

	r.Dispatch("ask", &app.Help{
		Summary: "Create or update an insensitive configuration value",
		Usage:   "vault-manager ask PATH NAME=[VALUE] [NAME ...]",
		Type:    app.DestructiveCommand,
		Description: `
Update a single path in the Vault with new or updated named attributes.
Any existing name/value pairs not specified on the command-line will
be left alone, with their original values.

You will be prompted to provide (without confirmation) any values that
are omitted. Unlike the 'vault-manager set' and 'vault-manager paste' commands, data entry
is NOT obscured.
`,
	}, func(command string, args ...string) error {
		return writeHelper(false, false, "ask", args...)
	})

	r.Dispatch("set", &app.Help{
		Summary: "Create or update a secret",
		Usage:   "vault-manager set PATH NAME=[VALUE] [NAME ...]",
		Type:    app.DestructiveCommand,
		Description: `
Update a single path in the Vault with new or updated named attributes.
Any existing name/value pairs not specified on the command-line will be
left alone, with their original values.

Values can be provided a number of different ways.

    vault-manager set secret/path key=value

Will set "key" to "value", but that exposes the value in the process table
(and possibly in shell history files).  This is normally fine for usernames,
IP addresses, and other public information.

If this worries you, leave off the '=value', and vault-manager will prompt you.

    vault-manager set secret/path key

Some secrets perfer to live on disk, in files.  Certificates, private keys,
really long secrets that are tough to type, etc.  For those, you can use
the '@' notation:

    vault-manager set secret/path key@path/to/file

This causes vault-manager to read the file 'path/to/file', relative to the current
working directory, and insert the contents into the Vault.
`,
	}, func(command string, args ...string) error {
		return writeHelper(true, true, "set", args...)
	})

	r.Dispatch("paste", &app.Help{
		Summary: "Create or update a secret",
		Usage:   "vault-manager paste PATH NAME=[VALUE] [NAME ...]",
		Type:    app.DestructiveCommand,
		Description: `
Works just like 'vault-manager set', updating a single path in the Vault with new or
updated named attributes.  Any existing name/value pairs not specified on the
command-line will be left alone, with their original values.

You will be prompted to provide any values that are omitted, but unlike the
'vault-manager set' command, you will not be asked to confirm those values.  This makes
sense when you are pasting in credentials from an external password manager
like 1password or Lastpass.
`,
	}, func(command string, args ...string) error {
		//Dispatch call.
		return writeHelper(false, true, "paste", args...)
	})

	r.Dispatch("exists", &app.Help{
		Summary: "Check to see if a secret exists in the Vault",
		Usage:   "vault-manager exists PATH",
		Type:    app.NonDestructiveCommand,
		Description: `
When you want to see if a secret has been defined, but don't need to know
what its value is, you can use 'vault-manager exists'.  PATH can either be a partial
path (i.e. 'secret/accounts/users/admin') or a fully-qualified path that
incudes a name (like 'secret/accounts/users/admin:username').

'vault-manager exists' does not produce any output, and is suitable for use in scripts.

The process will exit 0 (zero) if PATH exists in the current Vault.
Otherwise, it will exit 1 (one).  If unrelated errors, like network timeouts,
certificate validation failure, etc. occur, they will be printed as well.
`,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		if len(args) != 1 {
			r.ExitWithUsage("exists")
		}
		v := app.Connect(true)
		_, err := v.Read(args[0])
		if err != nil {
			if vault.IsNotFound(err) {
				os.Exit(1)
			}
			return err
		}
		os.Exit(0)
		return nil
	})

	r.Dispatch("get", &app.Help{
		Summary: "Retrieve the key/value pairs (or just keys) of one or more paths",
		Usage:   "vault-manager get [--keys] [--yaml] PATH [PATH ...]",
		Description: `
Allows you to retrieve one or more values stored in the given secret, or just the
valid keys.  It operates in the following modes:

If a single path is specified that does not include a :key suffix, the output
will be the key:value pairs for that secret, in YAML format.  It will not include
the specified path as the base hash key; instead, it will be output as a comment
behind the document indicator (---).  To force it to include the full path as
the root key, specify --yaml.

If a single path is specified including the :key suffix, the single value of that
path:key will be output in string format.  To force the use of the fully qualified
{path: {key: value}} output in YAML format, use --yaml option.

If a single path is specified along with --keys, the list of keys for that given
path will be returned.  If that path does not contain any secrets (ie its not a
leaf node or does not exist), it will output nothing, but will not error.  If a
specific key is specified, it will output only that key if it exists, otherwise
nothing. You can specify --yaml to force YAML output.

If you specify more than one path, output is forced to be YAML, with the primary
hash key being the requested path (not including the key if provided).  If --keys
is specified, the next level will contain the keys found under that path; if the
path included a key component, only the specified keys will be present.  Without
the --keys option, the key: values for each found (or requested) key for the path
will be output.

If an invalid key or path is requested, an error will be output and nothing else
unless the --keys option is specified.  In that case, the error will be displayed
as a warning, but the output will be provided with an empty array for missing
paths/keys.
`,
		Type: app.NonDestructiveCommand,
	}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		if len(args) < 1 {
			r.ExitWithUsage("get")
		}

		v := app.Connect(true)

		// Recessive case of one path
		if len(args) == 1 && !opt.Get.Yaml {
			s, err := v.Read(args[0])
			if err != nil {
				return err
			}

			if opt.Get.KeysOnly {
				keys := s.Keys()
				for _, key := range keys {
					fmt.Printf("%s\n", key)
				}
			} else if _, key, _ := vault.ParsePath(args[0]); key != "" {
				value, err := s.SingleValue()
				if err != nil {
					return err
				}
				fmt.Printf("%s\n", value)
			} else {
				fmt.Printf("--- # %s\n%s\n", args[0], s.YAML())
			}
			return nil
		}

		// Track errors, paths, keys, values
		errs := make([]error, 0)
		results := make(map[string]map[string]string, 0)
		missingKeys := make(map[string][]string)
		for _, path := range args {
			p, k, _ := vault.ParsePath(path)
			s, err := v.Read(path)

			// Check if the desired path[:key] is found
			if err != nil {
				errs = append(errs, err)
				if k != "" {
					if _, ok := missingKeys[p]; !ok {
						missingKeys[p] = make([]string, 0)
					}
					missingKeys[p] = append(missingKeys[p], k)
				}
				continue
			}

			if _, ok := results[p]; !ok {
				results[p] = make(map[string]string, 0)
			}
			for _, key := range s.Keys() {
				results[p][key] = s.Get(key)
			}
		}

		// Handle any errors encountered.  Warn for key request, return error otherwise
		var err error
		numErrs := len(errs)
		if numErrs == 1 {
			err = errs[0]
		} else if len(errs) > 1 {
			errStr := "Multiple errors found:"
			for _, err := range errs {
				errStr += fmt.Sprintf("\n   - %s", err)
			}
			err = errors.New(errStr)
		}
		if numErrs > 0 {
			if opt.Get.KeysOnly {
				fmt.Fprintf(os.Stderr, "@y{WARNING:} %s\n", err)
			} else {
				return err
			}
		}

		// Now that we've collected/collated all the data, format and print it
		fmt.Printf("---\n")
		if opt.Get.KeysOnly {
			printedPaths := make(map[string]bool, 0)
			for _, path := range args {
				p, _, _ := vault.ParsePath(path)
				if printed, _ := printedPaths[p]; printed {
					continue
				}
				printedPaths[p] = true
				result, ok := results[p]
				if !ok {
					yml, _ := yaml.Marshal(map[string][]string{p: []string{}})
					fmt.Printf("%s", string(yml))
				} else {
					foundKeys := reflect.ValueOf(result).MapKeys()
					strKeys := make([]string, len(foundKeys))
					for i := 0; i < len(foundKeys); i++ {
						strKeys[i] = foundKeys[i].String()
					}
					sort.Strings(strKeys)
					yml, _ := yaml.Marshal(map[string][]string{p: strKeys})
					fmt.Printf("%s\n", string(yml))
				}
			}
		} else {
			yml, _ := yaml.Marshal(results)
			fmt.Printf("%s\n", string(yml))
		}
		return nil
	})
}
