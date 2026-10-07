package vault

import (
	"fmt"
	"strings"

	"github.com/cloudfoundry-community/vaultkv"

	"github.com/SomeBlackMagic/vault-manager/logging"
)

const (
	verifyStateAlive uint = iota
	verifyStateAliveOrDeleted
)

type verifyOpts struct {
	AnyVersion bool
	State      uint
}

type DeleteOpts struct {
	Destroy bool
	All     bool
}

func (v *Vault) verifySecretState(path string, opts verifyOpts) error {
	secret, _, version := ParsePath(path)
	mountV, err := v.MountVersion(secret)
	if err != nil {
		return err
	}

	var deletedErr = secretNotFound{fmt.Sprintf("`%s' is deleted", path)}
	var destroyedErr = secretNotFound{fmt.Sprintf("`%s' is destroyed", path)}

	switch mountV {
	case 1:
		return v.verifySecretExists(path)
	case 2:
		versions, err := v.Versions(secret)
		if err != nil {
			if IsNotFound(err) {
				err = v.errIfFolder(path, "`%s' points to a folder, not a secret", path)
				if err != nil {
					return err
				}

				return NewSecretNotFoundError(secret)
			}

			return err
		}

		if !opts.AnyVersion {
			var v vaultkv.KVVersion
			if version == 0 {
				v = versions[len(versions)-1]
			} else {
				if uint64(versions[0].Version) > version {
					return destroyedErr
				}

				if version > uint64(versions[len(versions)-1].Version) {
					return secretNotFound{fmt.Sprintf("`%s' references a version that does not yet exist", path)}
				}

				idx := version - uint64(versions[0].Version)
				v = versions[idx]
			}

			if v.Destroyed {
				return destroyedErr
			}
			if opts.State == verifyStateAlive && v.Deleted {
				return deletedErr
			}
		} else {
			for i := range versions {
				if !(versions[i].Deleted || versions[i].Destroyed) || (opts.State == verifyStateAliveOrDeleted && !versions[i].Destroyed) {
					return nil
				}
			}

			//If we got this far, we couldn't find a version that satisfied our constraints
			if opts.State == verifyStateAlive {
				return secretNotFound{fmt.Sprintf("No living versions for `%s'", path)}
			} else {
				return secretNotFound{fmt.Sprintf("No living or deleted versions for `%s'", path)}
			}
		}

	default:
		return fmt.Errorf("Unsupported mount version: %d", mountV)
	}
	return nil
}

func (v *Vault) verifySecretExists(path string) error {
	path = Canonicalize(path)

	_, err := v.Read(path)
	if err != nil && IsNotFound(err) { //if this was not a leaf node (secret)...
		if folderErr := v.errIfFolder(path, "`%s` points to a folder, not a secret", path); folderErr != nil {
			return folderErr
		}
	}
	return err
}

func (v *Vault) verifyMetadataExists(path string) error {
	versions, err := v.Versions(path)
	if err != nil {
		if vaultkv.IsNotFound(err) {
			return NewSecretNotFoundError(path)
		}
		return err
	}

	if len(versions) == 0 {
		return NewSecretNotFoundError(path)
	}

	return nil
}

func (v *Vault) canSemanticallyDelete(path string) error {
	justSecret, key, version := ParsePath(path)
	if key == "" || version == 0 {
		return nil
	}

	versions, err := v.Versions(justSecret)
	if err != nil {
		return err
	}

	if versions[len(versions)-1].Version == uint(version) {
		return nil
	}

	s, err := v.Read(path)
	if err != nil {
		return err
	}

	if len(s.data) != 1 || !s.Has(key) {
		return fmt.Errorf("Cannot delete specific non-isolated key of non-latest version")
	}

	return nil
}

// Delete removes the secret or key stored at the specified path.
// If destroy is true and the mount is v2, the latest version is destroyed instead
func (v *Vault) Delete(path string, opts DeleteOpts) error {
	path = Canonicalize(path)
	v.Log().Debug("deleting secret", logging.KeyPath, path, "destroy", opts.Destroy, "all", opts.All)

	reqState := verifyStateAlive
	if opts.Destroy {
		reqState = verifyStateAliveOrDeleted
	}

	err := v.verifySecretState(path, verifyOpts{
		AnyVersion: opts.All,
		State:      reqState,
	})
	if err != nil {
		return err
	}

	err = v.canSemanticallyDelete(path)
	if err != nil {
		return err
	}

	if !PathHasKey(path) {
		return v.deleteEntireSecret(path, opts.Destroy, opts.All)
	}

	return v.deleteSpecificKey(path)
}

// DeleteTree recursively deletes the leaf nodes beneath the given root until
// the root has no children, and then deletes that.
func (v *Vault) DeleteTree(root string, opts DeleteOpts) error {
	root = Canonicalize(root)

	secrets, err := v.ConstructSecrets(root, TreeOpts{FetchKeys: false, SkipVersionInfo: true, AllowDeletedSecrets: true})
	if err != nil {
		return err
	}
	for _, path := range secrets.Paths() {
		err = v.deleteEntireSecret(path, opts.Destroy, opts.All)
		if err != nil {
			return err
		}
	}

	mount, err := v.Client().MountPath(root)
	if err != nil {
		return err
	}

	if strings.Trim(root, "/") != strings.Trim(mount, "/") {
		err = v.deleteEntireSecret(root, opts.Destroy, opts.All)
	}

	return err
}

// DeleteVersions marks the given versions of the given secret as deleted for
// a v2 backend or actually deletes it for a v1 backend.
func (v *Vault) DeleteVersions(path string, versions []uint) error {
	return v.client.Delete(path, &vaultkv.KVDeleteOpts{Versions: versions, V1Destroy: true})
}

// DestroyVersions irrevocably destroys the given versions of the given secret
func (v *Vault) DestroyVersions(path string, versions []uint) error {
	return v.client.Destroy(path, versions)
}

func (v *Vault) Undelete(path string) error {
	secret, key, version := ParsePath(path)
	if key != "" {
		return fmt.Errorf("Cannot undelete specific key (%s)", path)
	}

	respVersions, err := v.Versions(secret)
	if err != nil {
		return err
	}

	if version == 0 {
		version = uint64(respVersions[len(respVersions)-1].Version)
	}

	destroyedErr := fmt.Errorf("`%s' version: %d is destroyed", secret, version)
	firstVersion := respVersions[0].Version
	if uint(version) < firstVersion {
		return destroyedErr
	}

	idx := int(uint(version) - firstVersion)
	if idx >= len(respVersions) {
		return fmt.Errorf("version %d of `%s' does not yet exist", version, secret)
	}

	if respVersions[idx].Destroyed {
		return destroyedErr
	}

	return v.Client().Undelete(secret, []uint{uint(version)})
}

// deleteIfPresent first checks to see if there is a Secret at the given path,
// and if so, it deletes it. Otherwise, no error is thrown
func (v *Vault) deleteIfPresent(path string, opts DeleteOpts) error {
	secretpath, _, _ := ParsePath(path)
	if _, err := v.Read(secretpath); err != nil {
		if IsSecretNotFound(err) {
			return nil
		}
		return err
	}

	err := v.Delete(path, opts)
	if IsKeyNotFound(err) {
		return nil
	}
	return err
}

func (v *Vault) deleteEntireSecret(path string, destroy bool, all bool) error {
	secret, _, version := ParsePath(path)

	if destroy && all {
		return v.client.DestroyAll(secret)
	}

	var versions []uint
	if version != 0 {
		versions = []uint{uint(version)}
	}

	if destroy {
		allVersions, err := v.Versions(secret)
		if err != nil {
			return err
		}
		//Need to populate latest version to a Destroy call if the
		// version is not explicitly given
		if len(versions) == 0 {
			versions = []uint{allVersions[len(allVersions)-1].Version}
		}
		//Check if we should clean up the metadata entirely because there are
		// no more remaining non-destroyed versions
		shouldNuke := true
		verIdx := 0
		for i := range allVersions {
			for verIdx < len(versions) && versions[verIdx] < allVersions[i].Version {
				verIdx++
			}
			if !allVersions[i].Destroyed && (verIdx >= len(versions) || versions[verIdx] != allVersions[i].Version) {
				shouldNuke = false
				break
			}
		}

		if shouldNuke {
			return v.client.DestroyAll(secret)
		}
		return v.client.Destroy(secret, versions)
	}

	if all {
		allVersions, err := v.Versions(secret)
		if err != nil {
			return err
		}

		versions = make([]uint, 0, len(allVersions))
		for i := range allVersions {
			versions = append(versions, allVersions[i].Version)
		}

	}

	return v.client.Delete(secret, &vaultkv.KVDeleteOpts{Versions: versions, V1Destroy: true})
}

func (v *Vault) deleteSpecificKey(path string) error {
	secretPath, key, _ := ParsePath(path)
	secret, err := v.Read(secretPath)
	if err != nil {
		return err
	}
	deleted := secret.Delete(key)
	if !deleted {
		return NewKeyNotFoundError(secretPath, key)
	}
	if secret.Empty() {
		//Gotta avoid call to Write because Write ignores version information (with good reason)
		// We can only be here and not be on the latest version if this was the only key remaining
		// and we're just trying to nuke the secret
		//
		//At some point, we should probably get Destroy routed into here so that we can destroy
		// secrets through specifying keys
		return v.deleteEntireSecret(secretPath, false, false)
	}
	return v.Write(secretPath, secret)
}
