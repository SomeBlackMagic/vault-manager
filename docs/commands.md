Command Reference
==================

`vault-manager` operates by way of sub-commands. This page documents every
command exposed by the CLI, grouped by area.

- [Targets & Authentication](#targets--authentication)
- [Secrets](#secrets)
- [Generation](#generation)
- [Listing & Tree](#listing--tree)
- [Migration](#migration)
- [X.509 Certificates](#x509-certificates)
- [Admin](#admin)
- [Formatting & Misc](#formatting--misc)
- [Sync](#sync)

Targets & Authentication
-------------------------

### target [url] alias

Target a new Vault, or set your current Vault target.

```
vault-manager target https://vault.example.com myvault
vault-manager target myvault
```

The first argument is the URL to the Vault; the second is a shorthand alias
for the target. Once a target has been added, you can retarget it by alias
alone.

### targets

List all targeted Vaults, along with which one is currently active.

```
vault-manager targets
```

### target delete alias

Forget about a targeted Vault.

```
vault-manager target delete myvault
```

### auth [token|ldap|github|okta|userpass|approle]

Authenticate to the current target.

```
vault-manager auth [token]
vault-manager auth ldap
vault-manager auth github
vault-manager auth okta
```

For each type (token, ldap, okta, github, userpass, approle), you will be
prompted for the necessary credentials to authenticate against the Vault.

### logout

Forget the authentication token of the currently targeted Vault.

```
vault-manager logout
```

### renew [all]

Renew one or more authentication tokens.

```
vault-manager renew
vault-manager renew all
```

### env

Print the environment variables describing the current target:

```
vault-manager env
  VAULT_ADDR  http://localhost:8200
  VAULT_TOKEN  $SOME_UUID
```

You can also use this command to export a target's configuration into the
outer shell in order to use the Vault CLI directly:

```
vault-manager env --bash
\export VAULT_ADDR=http://localhost:8200;
\export VAULT_TOKEN=$SOME_UUID;
\unset VAULT_SKIP_VERIFY;

eval $(vault-manager env --bash)
```

Secrets
-------

### set path key\[=value\] \[key ...\]

Updates a single path with new keys. Any existing keys that are not
specified on the command line are left intact.

You will be prompted to enter values for any keys that do not have values.
This can be used for more sensitive credentials like passwords, PINs, etc.

Example:

```
vault-manager set secret/root username=root password
<prompts for 'password' here...>
```

Setting the value of a key to be the contents of a file:

```
vault-manager set secret/root ssl_key@/path/to/ssl_key_file
```

All operations (except for `delete`) are additive, so the following:

```
vault-manager set secret/x a=b c=d
```

is equivalent to this:

```
vault-manager set secret/x a=b -- set secret/x c=d
```

### paste path key\[=value\] \[key ...\]

Works the same way as `set`, but does not have a confirmation prompt for
your value. It assumes you have pasted in the value from a known-good
source. Useful if you use a password manager and don't want to paste
passwords twice:

```
vault-manager paste secret/1pass/managed
```

### ask path name\[=value\] \[name ...\]

Create or update an insensitive (non-secret) configuration value. Behaves
like `set`, but is intended for values that are not sensitive credentials.

```
vault-manager ask secret/config environment=production
```

### exists path

Check to see if a secret exists in the Vault. Useful for scripting.

```
vault-manager exists secret/root
```

### get path \[path ...\]

Retrieve and print the values of one or more paths, to standard output.
This is most useful for piping credentials through `keybase` or `pgp` for
encrypting and sending to others.

```
vault-manager get secret/root secret/whatever secret/key
--- # secret/root
username: root
password: it's a secret

--- # secret/whatever
whatever: is clever

--- # secret/key
private: |
   -----BEGIN RSA PRIVATE KEY-----
   ...
   -----END RSA PRIVATE KEY-----
public: |
  -----BEGIN RSA PUBLIC KEY-----
  ...
  -----END RSA PRIVATE KEY-----
```

### delete path \[path ...\]

Removes multiple paths from the Vault.

```
vault-manager delete secret/unused
```

### undelete path \[path ...\]

Undelete a soft-deleted secret from a Vault KV v2 backend.

```
vault-manager undelete secret/oops
```

Generation
----------

### gen \[length\] path key

Generate a new, random password. By default, the generated password will
be 64 characters long.

```
vault-manager gen secret/account secretkey
```

To get a shorter password, only 16 characters long:

```
vault-manager gen 16 secret/account password
```

### uuid path\[:key\]

Generate a new UUIDv4 and store it at the given path/key.

```
vault-manager uuid secret/account id
```

### ssh \[nbits\] path \[path ...\]

Generate a new SSH RSA keypair, adding the keys "private" and "public" to
each path. The public key will be encoded as an authorized keys entry. The
private key is a PEM-encoded DER private key.

By default, a 2048-bit key will be generated. The `nbits` parameter allows
you to change that.

Each path gets a unique SSH keypair.

```
vault-manager ssh 2048 secret/ssh
```

### rsa \[nbits\] path \[path ...\]

Generate a new RSA keypair, adding the keys "private" and "public" to each
path. Both keys will be PEM-encoded DER.

By default, a 2048-bit key will be generated. The `nbits` parameter allows
you to change that.

Each path gets a unique RSA keypair.

### dhparam \[nbits\] path

Generate Diffie-Hellman key exchange parameters and store them at the
given path.

```
vault-manager dhparam 2048 secret/tls/dhparam
```

Listing & Tree
--------------

### tree path \[path ...\]

Provide a tree hierarchy listing of all reachable keys in the Vault.

```
vault-manager tree secret/dc1
secret/dc1
  concourse/
    pipeline-the-first/
      aws
      dockerhub
      github
    pipeline-the-second/
      aws
      dockerhub
      github
```

### paths path \[path ... \]

Provide a flat listing of all reachable keys in the Vault.

```
vault-manager paths secret/dc1
secret/dc1concourse/pipeline-the-first/aws
secret/dc1concourse/pipeline-the-first/dockerhub
secret/dc1concourse/pipeline-the-first/github
secret/dc1concourse/pipeline-the-second/aws
secret/dc1concourse/pipeline-the-second/dockerhub
secret/dc1concourse/pipeline-the-second/github
```

### ls \[-1|-q\] \[path ...\]

Print the keys and sub-directories at one or more paths (non-recursive,
single-level listing).

```
vault-manager ls secret/dc1/concourse
```

### versions path \[path ...\]

Print information about the versions of one or more paths (for Vault KV v2
backends).

```
vault-manager versions secret/root
```

Migration
---------

### move oldpath newpath

Move a secret from `oldpath` to `newpath`, a rename of sorts.

```
vault-manager move secret/staging/user secret/prod/user
```

(or, more succinctly, using brace expansion):

```
vault-manager move secret/{staging,prod}/user
```

Any credentials at `newpath` will be completely overwritten. The secret at
`oldpath` will no longer exist.

### copy oldpath newpath

Copy a secret from `oldpath` to `newpath`.

```
vault-manager copy secret/staging/user secret/prod/user
```

(or, as with `move`, using brace expansion):

```
vault-manager copy secret/{staging,prod}/user
```

Any credentials at `newpath` will be completely overwritten. The secret at
`oldpath` will still exist after the copy.

### revert path version

Revert a secret to a previous version (Vault KV v2 backends).

```
vault-manager revert secret/root 3
```

### export path \[path ...\]

Export the given subtree(s) in a format suitable for migration (via
`import`), or long-term storage offline. Secrets will not be encrypted in
this representation, so care should be taken in handling it. Output will
be printed to standard output.

### import <export.file

Read an export (as produced by the `export` subcommand) from standard
input, and write all of the secrets contained therein to the same paths
inside the targeted Vault. Trees will be imported in an additive nature,
so existing credentials in the same subtree as imported credentials will
be left intact.

If you've got an export saved in a file _on-disk_, you can feed it to
`vault-manager import` using your shell's redirection facilities:

```
vault-manager import < ./path/to/export.file
```

You can also use `cat`, in the standard UNIX idiom:

```
cat ./path/to/export.file | vault-manager import
```

(_Note:_ storing exports on-disk is considered bad practice, as it leaks
your secrets via a shared resource: the filesystem.)

Import and export can be combined in a pipeline to facilitate movement of
credentials from one Vault to another, like so:

```
vault-manager -T old-vault export secret/sub/tree | \
  vault-manager -T new-vault import
```

X.509 Certificates
-------------------

### x509 issue \[OPTIONS\] --name cn.example.com path

Issues a new X.509 TLS/SSL certificate, and stores the new RSA private key
and the certificate in the Vault at _path_, in PEM format.

### x509 reissue \[OPTIONS\] path

Reissue an existing X.509 certificate, generating a new key/certificate
pair in its place.

```
vault-manager x509 reissue secret/tls/example
```

### x509 renew \[OPTIONS\] path

Renew an existing X.509 certificate in place, keeping the same key.

```
vault-manager x509 renew secret/tls/example
```

### x509 revoke \[OPTIONS\] --signed-by path/to/ca path/to/cert

Revoke a certificate that was signed by a Certificate Authority. The
private key for the CA must be present in the Vault for this to work.
Revoked certificates will be appended to the CA's certificate revocation
list (CRL), stored at `path/to/ca:crl`.

### x509 validate \[OPTIONS\] path

Run a variety of validation checks against a certificate in the Vault. In
its simplest form, without arguments, this verifies that the private key
stored at `path:key` matches the certificate stored at `path:certificate`.
Options control more powerful validations, like checking for revocation,
SAN validity, and expiry.

### x509 show path \[path ...\]

Show the details (subject, issuer, validity window, SANs, etc.) of an
X.509 certificate stored in the Vault.

```
vault-manager x509 show secret/tls/example
```

### x509 crl --renew path

Renews (re-signs) the certificate authority at `path`, without affecting
the list of revoked certificates.

Admin
-----

These commands operate on the Vault server/cluster itself, rather than on
individual secrets, and typically require elevated privileges.

### status

Print the status of the current target's backend nodes.

```
vault-manager status
```

### init \[--keys #\] \[--threshold #\] \[--single\] \[--json\] \[--no-mount\] \[--sealed\]

Initialize a new Vault.

```
vault-manager init --keys 5 --threshold 3
```

### unseal

Unseal the current target.

```
vault-manager unseal
```

### seal

Seal the current target.

```
vault-manager seal
```

### rekey \[--gpg email@address ...\] \[--keys #\] \[--threshold #\]

Re-key your Vault with new unseal keys. **This is a destructive
operation** — make sure you understand the implications before running
it.

```
vault-manager rekey --keys 5 --threshold 3
```

### local (--memory|--file path/to/dir) \[--as name\] \[--port port\]

Run a local (dev) Vault, useful for testing `vault-manager` itself or
trying out workflows without a real Vault cluster.

```
vault-manager local --memory
```

Formatting & Misc
------------------

### fmt format_type path oldKey newKey

Take the key at `path:oldKey`, reformat it according to **format_type**,
and save it in `path:newKey`. Useful for hashing, or encoding passwords in
an alternate format (for htpasswd files, or `/etc/shadow`).

Currently supported formats:

- base64
- bcrypt
- crypt-md5
- crypt-sha256
- crypt-sha512

```
vault-manager gen secret/account password
vault-manager fmt base64 secret/account password base64_pass
vault-manager fmt crypt-sha512 secret/account password crypt_pass
vault-manager get secret/account
```

### prompt ...

Echo the arguments, space-separated, as a single line to the terminal.
This is a convenience helper for long pipelines of chained commands.

### option \[optionname=value\]

View or edit global `vault-manager` CLI options.

```
vault-manager option
vault-manager option clobber=true
```

### vault ...

Run arbitrary Vault CLI commands against the current target, using
`vault-manager`'s own authentication/target configuration.

```
vault-manager vault kv list secret/
```

### curl \[OPTIONS\] METHOD REL-URI \[DATA\]

Issue arbitrary HTTP requests to the current Vault, for diagnostics.

```
vault-manager curl GET /v1/sys/health
```

Sync
----

`sync pull`, `sync plan`, and `sync apply` synchronize secrets between
Vault and a local directory of JSON files. See [sync.md](sync.md) for the
full reference, including the local JSON file format.
