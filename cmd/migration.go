package cmd

import (
	"encoding/json"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	fmt "github.com/jhunt/go-ansi"
	"github.com/SomeBlackMagic/vault-manager/app"
	"github.com/SomeBlackMagic/vault-manager/prompt"
	"github.com/SomeBlackMagic/vault-manager/rc"
	"github.com/SomeBlackMagic/vault-manager/vault"
)

// For versions of vault-manager 0.10+
// Older versions just use a map[string]map[string]string
type exportFormat struct {
	ExportVersion uint `json:"export_version"`
	//map from path string to map from version number to version info
	Data               map[string]exportSecret `json:"data"`
	RequiresVersioning map[string]bool         `json:"requires_versioning"`
}

type exportSecret struct {
	FirstVersion uint            `json:"first,omitempty"`
	Versions     []exportVersion `json:"versions"`
}

type exportVersion struct {
	Deleted   bool              `json:"deleted,omitempty"`
	Destroyed bool              `json:"destroyed,omitempty"`
	Value     map[string]string `json:"value,omitempty"`
}

func registerMigrationCommands(r *app.Runner, opt *Options) {
	r.Dispatch("delete", &app.Help{
		Summary: "Remove one or more path from the Vault",
		Usage:   "vault-manager delete [-rfDa] PATH [PATH ...]",
		Type:    app.DestructiveCommand,
		Description: `
-d (--destroy) will cause KV v2 secrets to be destroyed instead of
being marked as deleted. For KV v1 backends, this would do nothing.
-a (--all) will delete (or destroy) all versions of the secret instead
of just the specified (or latest if unspecified) version.
`}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)

		if len(args) < 1 {
			r.ExitWithUsage("delete")
		}
		v := app.Connect(true)

		verb := "delete"
		if opt.Delete.Destroy {
			verb = "destroy"
		}

		for _, path := range args {
			_, key, version := vault.ParsePath(path)

			//Ignore -r if path has a version or key because that seems like a mistake
			if opt.Delete.Recurse && (key == "" || version > 0) {
				if !opt.Delete.Force && !recursively(verb, path) {
					continue /* skip this command, process the next */
				}
				if err := v.DeleteTree(path, vault.DeleteOpts{
					Destroy: opt.Delete.Destroy,
					All:     opt.Delete.All,
				}); err != nil && !(vault.IsNotFound(err) && opt.Delete.Force) {
					return err
				}
			} else {
				if err := v.Delete(path, vault.DeleteOpts{
					Destroy: opt.Delete.Destroy,
					All:     opt.Delete.All,
				}); err != nil && !(vault.IsNotFound(err) && opt.Delete.Force) {
					return err
				}
			}
		}
		return nil
	})

	r.Dispatch("undelete", &app.Help{
		Summary: "Undelete a soft-deleted secret from a V2 backend",
		Usage:   "vault-manager undelete PATH [PATH ...]",
		Type:    app.DestructiveCommand,
		Description: `
If no version is specified, this attempts to undelete the newest version of the secret
This does not error if the specified version exists but is not deleted
This errors if the secret or version does not exist, of if the particular version has
been irrevocably destroyed. An error also occurs if a key is specified.

-a (--all) undeletes all versions of the given secret.
`}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)

		if len(args) < 1 {
			r.ExitWithUsage("undelete")
		}
		v := app.Connect(true)

		for _, path := range args {
			var err error
			if opt.Undelete.All {
				secret, key, version := vault.ParsePath(path)
				if key != "" {
					return fmt.Errorf("Cannot undelete specific key (%s)", path)
				}

				if version > 0 {
					return fmt.Errorf("--all given but path (%s) has version specified", path)
				}

				respVersions, err := v.Versions(secret)
				if err != nil {
					return err
				}

				versions := make([]uint, 0, len(respVersions))
				for _, v := range respVersions {
					versions = append(versions, v.Version)
				}

				err = v.Client().Undelete(path, versions)
			} else {
				err = v.Undelete(path)
			}
			if err != nil {
				return err
			}
		}

		return nil
	})

	r.Dispatch("revert", &app.Help{
		Summary: "Revert a secret to a previous version",
		Usage:   "vault-manager revert PATH VERSION",
		Type:    app.DestructiveCommand,
		Description: `
-d (--deleted) will handle deleted versions by undeleting them, reading them, and then
redeleting them.
`}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		if len(args) != 2 {
			r.ExitWithUsage("revert")
		}
		v := app.Connect(true)

		secret, key, version := vault.ParsePath(args[0])
		if key != "" {
			return fmt.Errorf("Cannot call revert with path containing key")
		}

		if version > 0 {
			return fmt.Errorf("Cannot call revert with path containing version")
		}

		targetVersion, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("VERSION must be a positive integer")
		}

		if targetVersion == 0 {
			return nil
		}

		//Check what the most recent version is to avoid setting the latest version if unnecessary.
		// This should also catch if the secret is non-existent, or if we're targeting a destroyed,
		// deleted, or non-existent version.
		allVersions, err := v.Versions(args[0])
		if err != nil {
			return err
		}

		destroyedErr := fmt.Errorf("Version %d of secret `%s' is destroyed", targetVersion, secret)
		if targetVersion < uint64(allVersions[0].Version) {
			return destroyedErr
		}

		if targetVersion > uint64(allVersions[len(allVersions)-1].Version) {
			return fmt.Errorf("Version %d of secret `%s' does not exist", targetVersion, secret)
		}

		versionObject := allVersions[targetVersion-uint64(allVersions[0].Version)]
		if versionObject.Destroyed {
			return destroyedErr
		}

		if versionObject.Deleted {
			if !opt.Revert.Deleted {
				return fmt.Errorf("Version %d of secret `%s' is deleted. To force a read, specify --deleted", targetVersion, secret)
			}

			err = v.Undelete(vault.EncodePath(secret, "", targetVersion))
			if err != nil {
				return err
			}
		}

		//If the version to revert to is the current version, do nothing...
		// unless its deleted, then either just undelete it or err, depending on
		// if the -d flag is set
		if targetVersion == uint64(allVersions[len(allVersions)-1].Version) {
			return nil
		}

		toWrite, err := v.Read(vault.EncodePath(secret, "", targetVersion))
		if err != nil {
			return err
		}

		err = v.Write(secret, toWrite)
		if err != nil {
			return err
		}

		//If we got this far and this is set, we must have undeleted a thing.
		// Clean up after ourselves
		if versionObject.Deleted {
			err = v.Delete(vault.EncodePath(secret, "", targetVersion), vault.DeleteOpts{})
			if err != nil {
				return err
			}
		}

		return nil
	})

	r.Dispatch("export", &app.Help{
		Summary: "Export one or more subtrees for migration / backup purposes",
		Usage:   "vault-manager export [-ad] PATH [PATH ...]",
		Type:    app.NonDestructiveCommand,
		Description: `
Normally, the export will get only the latest version of each secret, and encode it in a format that is backwards-
compatible with pre-1.0.0 versions of vault-manager (and newer versions).
-a (--all) will encode all versions of each secret. This will cause the export to use the V2 format, which is
incompatible with versions of vault-manager prior to v1.0.0
-d (--deleted) will cause vault-manager to undelete, read, and then redelete deleted secrets in order to encode them in the
backup. Without this, deleted versions will be ignored.
`}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		if len(args) < 1 {
			args = append(args, "secret")
		}
		v := app.Connect(true)

		var toExport interface{}

		//Standardize and validate paths
		for i := range args {
			args[i] = vault.Canonicalize(args[i])
			_, key, version := vault.ParsePath(args[i])
			if key != "" {
				return fmt.Errorf("Cannot export path with key (%s)", args[i])
			}

			if version > 0 {
				return fmt.Errorf("Cannot export path with version (%s)", args[i])
			}
		}

		//Deduplicate the input paths
		sort.Slice(args, func(i, j int) bool { return vault.PathLessThan(args[i], args[j]) })
		for i := 0; i < len(args)-1; i++ {
			//No need to get a deeper part of a tree if you're already walking the `((great)*grand)?parent`
			if strings.HasPrefix(strings.Trim(args[i+1], "/"), strings.Trim(args[i], "/")) {
				before := args[:i+1]
				var after []string
				if len(args)-1 != i+1 {
					after = args[i+2:]
				}
				args = append(before, after...)
				i--
			}
		}

		secrets := vault.Secrets{}
		for _, path := range args {
			theseSecrets, err := v.ConstructSecrets(path, vault.TreeOpts{
				FetchKeys:           true,
				FetchAllVersions:    opt.Export.All,
				GetDeletedVersions:  opt.Export.Deleted,
				AllowDeletedSecrets: opt.Export.Deleted,
			})
			if err != nil {
				return err
			}

			secrets = secrets.Merge(theseSecrets)
		}

		var mustV2Export bool
		//Determine if we can get away with a v1 export
		for _, s := range secrets {
			if len(s.Versions) > 1 {
				mustV2Export = true
				break
			}
		}

		v1Export := func() error {
			export := make(map[string]*vault.Secret)
			for _, s := range secrets {
				export[s.Path] = s.Versions[0].Data
			}

			toExport = export
			return nil
		}

		v2Export := func() error {
			export := exportFormat{ExportVersion: 2, Data: map[string]exportSecret{}, RequiresVersioning: map[string]bool{}}

			for _, secret := range secrets {
				if len(secret.Versions) > 1 {
					mount, _ := v.Client().MountPath(secret.Path)
					export.RequiresVersioning[mount] = true
				}

				thisSecret := exportSecret{FirstVersion: secret.Versions[0].Number}
				//We want to omit the `first` key in the json if it's 1
				if thisSecret.FirstVersion == 1 || opt.Export.Shallow {
					thisSecret.FirstVersion = 0
				}

				for _, version := range secret.Versions {
					thisVersion := exportVersion{
						Deleted:   version.State == vault.SecretStateDeleted && opt.Export.Deleted,
						Destroyed: version.State == vault.SecretStateDestroyed || (version.State == vault.SecretStateDeleted && !opt.Export.Deleted),
						Value:     map[string]string{},
					}

					for _, key := range version.Data.Keys() {
						thisVersion.Value[key] = version.Data.Get(key)
					}

					thisSecret.Versions = append(thisSecret.Versions, thisVersion)
				}

				export.Data[secret.Path] = thisSecret

				//Wrap export in array so that older versions of vault-manager don't try to import this improperly.
				toExport = []exportFormat{export}
			}

			return nil
		}

		var err error
		if mustV2Export {
			err = v2Export()
		} else {
			err = v1Export()
		}

		if err != nil {
			return err
		}
		b, err := json.Marshal(&toExport)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", string(b))

		return nil
	})

	r.Dispatch("import", &app.Help{
		Summary: "Import name/value pairs into the current Vault",
		Usage:   "vault-manager import <backup/file.json",
		Type:    app.DestructiveCommand,
		Description: `
-I (--ignore-destroyed) will keep destroyed versions from being replicated in the import by
rting garbage data and then destroying it (which is originally done to preserve version numbering).
-i (--ignore-deleted) will ignore deleted versions from being written during the import.
-s (--shallow) will write only the latest version for each secret.
`}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}

		if opt.SkipIfExists {
			fmt.Fprintf(os.Stderr, "@R{!!} @C{--no-clobber} @R{is incompatible with} @C{vault-manager import}\n")
			r.ExitWithUsage("import")
		}

		v := app.Connect(true)

		type importFunc func([]byte) error

		v1Import := func(input []byte) error {
			var data map[string]*vault.Secret
			err := json.Unmarshal(input, &data)
			if err != nil {
				return err
			}
			for path, s := range data {
				err = v.Write(path, s)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "wrote %s\n", path)
			}
			return nil
		}

		v2Import := func(input []byte) error {
			var unmarshalTarget []exportFormat
			err := json.Unmarshal(input, &unmarshalTarget)
			if err != nil {
				return fmt.Errorf("Could not interpret export file: %w", err)
			}

			if len(unmarshalTarget) != 1 {
				return fmt.Errorf("Improperly formatted export file")
			}

			data := unmarshalTarget[0]

			if !opt.Import.Shallow {
				//Verify that the mounts that require versioning actually support it. We
				//can't really detect if v1 mounts exist at this stage unless we assume
				//the token given has mount listing privileges. Not a big deal, because
				//it will become very apparent once we start trying to put secrets in it
				for mount, needsVersioning := range data.RequiresVersioning {
					if needsVersioning {
						mountVersion, err := v.MountVersion(mount)
						if err != nil {
							return fmt.Errorf("Could not determine existing mount version: %w", err)
						}

						if mountVersion != 2 {
							return fmt.Errorf("Export for mount `%s' has secrets with multiple versions, but the mount either\n"+
								"does not exist or does not support versioning", mount)
						}
					}
				}
			}

			//Put the secrets in the places, writing the versions in the correct order and deleting/destroying secrets that
			// need to be deleted/destroyed.
			for path, secret := range data.Data {
				s := vault.SecretEntry{
					Path: path,
				}

				firstVersion := secret.FirstVersion
				if firstVersion == 0 {
					firstVersion = 1
				}

				if opt.Import.Shallow {
					secret.Versions = secret.Versions[len(secret.Versions)-1:]
				}
				for i := range secret.Versions {
					state := vault.SecretStateAlive
					if secret.Versions[i].Destroyed {
						if opt.Import.IgnoreDestroyed {
							continue
						}
						state = vault.SecretStateDestroyed
					} else if secret.Versions[i].Deleted {
						if opt.Import.IgnoreDeleted {
							continue
						}
						state = vault.SecretStateDeleted
					}
					data := vault.NewSecret()
					for k, v := range secret.Versions[i].Value {
						data.Set(k, v, false)
					}
					s.Versions = append(s.Versions, vault.SecretVersion{
						Number: firstVersion + uint(i),
						State:  state,
						Data:   data,
					})
				}

				err := s.Copy(v, s.Path, vault.TreeCopyOpts{
					Clear: true,
					Pad:   !(opt.Import.IgnoreDestroyed || opt.Import.Shallow),
				})
				if err != nil {
					return err
				}
			}

			return nil
		}

		var fn importFunc
		//determine which version of the export format this is
		var typeTest interface{}
		json.Unmarshal(b, &typeTest)
		switch v := typeTest.(type) {
		case map[string]interface{}:
			fn = v1Import
		case []interface{}:
			if len(v) == 1 {
				if meta, isMap := (v[0]).(map[string]interface{}); isMap {
					version, isFloat64 := meta["export_version"].(float64)
					if isFloat64 && version == 2 {
						fn = v2Import
					}
				}
			}
		}

		if fn == nil {
			return fmt.Errorf("Unknown export file format - aborting")
		}

		return fn(b)
	})

	r.Dispatch("move", &app.Help{
		Summary: "Move a secret from one path to another",
		Usage:   "vault-manager move [-rfd] OLD-PATH NEW-PATH",
		Type:    app.DestructiveCommand,
		Description: `
Specifying the --deep (-d) flag will cause versions to be grabbed from the source
and overwrite all versions of the secret at the destination.
`}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)
		if len(args) != 2 {
			r.ExitWithUsage("move")
		}

		v := app.Connect(true)
		if vault.PathHasKey(args[0]) || vault.PathHasKey(args[1]) {
			if opt.Move.Deep {
				return fmt.Errorf("Cannot deep copy a specific key")
			}

			if !vault.PathHasKey(args[0]) && vault.PathHasKey(args[1]) {
				return fmt.Errorf("Cannot move from entire secret into specific key")
			}
		}

		if vault.PathHasVersion(args[1]) {
			return fmt.Errorf("Cannot move to a specific destination version")
		}

		//Don't try to recurse if operating on a key
		// args[0] is the source path. args[1] is the destination path.
		if opt.Move.Recurse && !(vault.PathHasKey(args[0]) || vault.PathHasKey(args[1])) {
			if !opt.Move.Force && !recursively("move", args...) {
				return nil /* skip this command, process the next */
			}
			err := v.MoveCopyTree(args[0], args[1], v.Move, vault.MoveCopyOpts{
				SkipIfExists: opt.SkipIfExists, Quiet: opt.Quiet, Deep: opt.Move.Deep, DeletedVersions: opt.Move.Deep,
			})
			if err != nil && !(vault.IsNotFound(err) && opt.Move.Force) {
				return err
			}
		} else {
			err := v.Move(args[0], args[1], vault.MoveCopyOpts{
				SkipIfExists: opt.SkipIfExists, Quiet: opt.Quiet, Deep: opt.Move.Deep, DeletedVersions: opt.Move.Deep,
			})
			if err != nil && !(vault.IsNotFound(err) && opt.Move.Force) {
				return err
			}
		}
		return nil
	})

	r.Dispatch("copy", &app.Help{
		Summary: "Copy a secret from one path to another",
		Usage:   "vault-manager copy [-rfd] OLD-PATH NEW-PATH",
		Type:    app.DestructiveCommand,
		Description: `
Specifying the --deep (-d) flag will cause all living versions to be grabbed from the source
and overwrite all versions of the secret at the destination.
`}, func(command string, args ...string) error {
		rc.Apply(opt.UseTarget)

		if len(args) != 2 {
			r.ExitWithUsage("copy")
		}
		v := app.Connect(true)

		if vault.PathHasKey(args[0]) || vault.PathHasKey(args[1]) {
			if opt.Copy.Deep {
				return fmt.Errorf("Cannot deep copy a specific key")
			}

			if !vault.PathHasKey(args[0]) && vault.PathHasKey(args[1]) {
				return fmt.Errorf("Cannot move from entire secret into specific key")
			}
		}

		if vault.PathHasVersion(args[1]) {
			return fmt.Errorf("Cannot copy to a specific destination version")
		}

		if opt.Copy.Recurse && vault.PathHasVersion(args[0]) {
			return fmt.Errorf("Cannot recursively copy a path with specific version")
		}

		//Don't try to recurse if operating on a key
		// args[0] is the source path. args[1] is the destination path.
		if opt.Copy.Recurse && !(vault.PathHasKey(args[0]) || vault.PathHasKey(args[1])) {
			if !opt.Copy.Force && !recursively("copy", args...) {
				return nil /* skip this command, process the next */
			}
			err := v.MoveCopyTree(args[0], args[1], v.Copy, vault.MoveCopyOpts{
				SkipIfExists:    opt.SkipIfExists,
				Quiet:           opt.Quiet,
				Deep:            opt.Copy.Deep,
				DeletedVersions: opt.Copy.Deep,
			})
			if err != nil && !(vault.IsNotFound(err) && opt.Copy.Force) {
				return err
			}
		} else {
			err := v.Copy(args[0], args[1], vault.MoveCopyOpts{
				SkipIfExists:    opt.SkipIfExists,
				Quiet:           opt.Quiet,
				Deep:            opt.Copy.Deep,
				DeletedVersions: opt.Copy.Deep,
			})
			if err != nil && !(vault.IsNotFound(err) && opt.Copy.Force) {
				return err
			}
		}
		return nil
	})
}

func recursively(cmd string, args ...string) bool {
	y := prompt.Normal("Recursively @R{%s} @C{%s} @Y{(y/n)} ", cmd, strings.Join(args, " "))
	y = strings.TrimSpace(y)
	return y == "y" || y == "yes"
}
