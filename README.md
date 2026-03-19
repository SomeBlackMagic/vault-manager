vault-manager - A Vault CLI
==================

Questions? Pop in our [slack channel](https://cloudfoundry.slack.com/messages/vault/)!

![vault-manager](docs/vault-manager.png)

[Vault][vault] is an awesome project and it comes with superb
documentation, a rock-solid server component and a flexible and
capable command-line interface.

![get-set-passwords](docs/safely-generate-passwords.gif)

So, why `vault-manager`?  To solve the following problems:

  1. Securely generate new SSH public / private keys
  2. Securely generate random RSA key pairs
  3. Auto-generate secure, random passwords
  4. Securely provide credentials, without files
  5. Dumping multiple paths

Primarily, these are things encountered in trying to build secure
BOSH deployments using Vault and [Spruce][spruce].

ATTENTION HOMEBREW USERS
------------------------

If you run Homebrew on MacOS, be aware that the the Formula for
vault-manager in homebrew core is outdated, incorrect, and unmaintained.
We maintain our own tap, which you are encouraged to use instead:

```
brew tap starkandwayne/cf
brew install starkandwayne/cf/vault-manager
```


Authentication
--------------

To make it easier to target multiple Vaults from one client (i.e.
your work laptop), `vault-manager` lets you track and authenticate against
_targets_, each representing a different vault.

To get started, you'll need to add a new target:

```
vault-manager target https://vault.example.com myvault
```

The first argument is the URL to the Vault; the second is a
shorthand alias for the target.  Later, you can retarget this
Vault with just:

```
vault-manager target myvault
```

You can see what Vaults you have targeted by running

```
vault-manager targets
```

All commands will be run against the currently targeted Vault.

To authenticate:

```
vault-manager auth [token]
vault-manager auth ldap
vault-manager auth github
vault-manager auth okta
```

(Other authentication backends are not yet supported)

For each type (token, ldap, okta or github), you will be prompted for
the necessary credentials to authenticated against the Vault.

Usage
-----

`vault-manager` operates by way of sub-commands.  To generate a new
2048-bit SSH keypair, and store it in `secret/ssh`:

```
vault-manager ssh 2048 secret/ssh
```

To set non-sensitive keys, you can just specify them inline:

```
vault-manager set secret/ssh username=system
```

If you use a password manager (good for you!) and don't want to
have to paste passwords twice, use the `paste` subcommand:

```
vault-manager paste secret/1pass/managed
```

Commands can be chained by separating them with the argument
terminator, `--`, so to both create a new SSH keypair and set the
username:

```
vault-manager ssh 2048 secret/ssh -- set secret/ssh username=system
```

Auto-generated passwords are easy too:

```
vault-manager gen secret/account passphrase
```

Sometimes, you just want to import passwords from another source
(like your own password manager), without the hassle of writing
files to disk or the risk of leaking credentials via the process
table or your shell history file.  For that, `vault-manager` provides a
double-confirmation interactive mode:

```
vault-manager set secret/ssl/ca passphrase
passphrase [hidden]:
passphrase [confirm]:
```

What you type will not be echoed back to the screen, and the
confirmation prompt is there to make sure your fingers didn't
betray you.

All operations (except for `delete`) are additive, so the
following:

```
vault-manager set secret/x a=b c=d
```

is equivalent to this:

```
vault-manager set secret/x a=b -- set secret/x c=d
```

Need to take an existing password, and generate a crypt-sha512 hash,
or base64 encode it? `vault-manager fmt` will do this, and store the results
in a new key for you, making it easy to generate a password, and then
format that password as needed.

```
vault-manager gen secret/account password
vault-manager fmt base64 secret/account password base64_pass
vault-manager fmt crypt-sha512 secret/account password crypt_pass
vault-manager get secret/account
```

Command Reference
------------------

### set path key\[=value\] \[key ...\]

Updates a single path with new keys.  Any existing keys that are
not specified on the command line are left intact.

You will be prompted to enter values for any keys that do not have
values.  This can be used for more sensitive credentials like
passwords, PINs, etc.

Example:

```
vault-manager set secret/root username=root password
<prompts for 'password' here...>
```

Similarly, `vault-manager paste` works the same way, but does not have a confirmation
prompt for your value. It assumes you have pasted in the value from a known-good
source.

Setting the value of a key to be the contents of a file

Example:

```
vault-manager set secret/root ssl_key@/path/to/ssl_key_file
```

### get path \[path ...\]

Retrieve and print the values of one or more paths, to standard
output.  This is most useful for piping credentials through
`keybase` or `pgp` for encrypting and sending to others.

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

### tree path \[path ...\]

Provide a tree hierarchy listing of all reachable keys in the
Vault.

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

### delete path \[path ...\]

Removes multiple paths from the Vault.

```
vault-manager delete secret/unused
```

### move oldpath newpath

Move a secret from `oldpath` to `newpath`, a rename of sorts.

```
vault-manager move secret/staging/user secret/prod/user
```

(or, more succinctly, using brace expansion):

```
vault-manager move secret/{staging,prod}/user
```

Any credentials at `newpath` will be completely overwritten.  The
secret at `oldpath` will no longer exist.

### copy oldpath newpath

Copy a secret from `oldpath` to `newpath`.

```
vault-manager copy secret/staging/user secret/prod/user
```

(or, as with `move`, using brace expansion):

```
save copy secret/{staging,prod}/user
```

Any credentials at `newpath` will be completely overwritten.  The
secret at `oldpath` will still exist after the copy.

### gen \[length\] path key

Generate a new, random password.  By default, the generated
password will be 64 characters long.

```
vault-manager gen secret/account secretkey
```

To get a shorter password, only 16 characters long:

```
vault-manager gen 16 secret/account password
```

### fmt format_type path oldKey newKey

Take the key at `path:oldKey`, reformat it according to **format_type**,
and save it in `path:newKey`. Useful for hashing, or encoding passwords
in an alternate format (for htpass files, or /etc/shadow).

Currently supported formats:

- base64
- bcrypt
- crypt-md5
- crypt-sha256
- crypt-sha512

```
vault-manager fmt base64 secret/account password base64_password
vault-manager fmt crypt-sha512 secret/account password crypt_password
```

### ssh \[nbits\] path \[path ...\]

Generate a new SSH RSA keypair, adding the keys "private" and
"public" to each path.  The public key will be encoded as an
authorized keys.  The private key is a PEM-encoded DER private
key.

By default, a 2048-bit key will be generated.  The `nbits`
parameter allows you to change that.

Each path gets a unique SSH keypair.

### rsa \[nbits\] path \[path ...\]

Generate a new RSA keypair, adding the keys "private" and "public"
to each path.  Both keys will be PEM-encoded DER.

By default, a 2048-bit key will be generated.  The `nbits`
parameter allows you to change that.

Each path gets a unique RSA keypair.

### prompt ...

Echo the arguments, space-separated, as a single line to the
terminal.  This is a convenience helper for long pipelines of
chained commands.

### x509 issue \[OPTIONS\] --name cn.example.com path

Issues a new X.509 TLS/SSL certificate, and stores the new RSA
private key and the certificate in the Vault at _path_, in PEM
format.

### x509 revoke \[OPTIONS\] --signed-by path/to/ca path/to/cert

Revoke a certificate that was signed by a Certificate Authority.
The private key for the CA must be present in the Vault for this
to work.  Revoked certificates will be appended to the CA's
certificate revocation list (CRL), stored at `path/to/ca:crl`

### x509 validate \[OPTIONS\] path

Run a variety of validation checks against a certificate in the
Vault.  In its simplest form, without arguments, this verifies
that the private key stored at `path:key` matches the certificate
stored at `path:certificate`.  Options control more powerful
validations, like checking for revocation, SAN validity, and
expiry.

### x509 crl --renew path

Renews (re-signs) the certificate authority at `path`, without
affecting the list of revoked certificates.

### export path \[path ...\]

Export the given subtree(s) in a format suitable for migration
(via a future `import` call), or long-term storage offline.
Secrets will not be encrypted in this representation, so care
should be taken in handling it.  Output will be printed to
standard output.

### import <export.file

Read an export (as produced by the `export` subcommand) from
standard input, and write all of the secrets contained
therein to the same paths inside the targeted Vault.  Trees
will be imported in an additive nature, so existing credentials
in the same subtree as imported credentials will be left intact.

If you've got an export saved in a file _on-disk_, you can feed
it to `vault-manager import` using your shell's redirection facilities:

```
vault-manager import < ./path/to/export.file
```

You can also use `cat`, in the standard UNIX idiom:

```
cat ./path/to/export.file | vault-manager import
```

(_Note:_ storing exports on-disk is considered bad practice, as
 it leaks your secrets via a shared resource: the filesystem.)

Import and export can be combined in a pipeline to facilitate
movement of credentials from one Vault to another, like so:

```
vault-manager -T old-vault export secret/sub/tree | \
  vault-manager -T new-vault import
```

### env

Print the environment variables describing the current target:

```
vault-manager env
  VAULT_ADDR  http://localhost:8200
  VAULT_TOKEN  $SOME_UUID
```

You can also use this command to export a target's configuration into the outer
shell in order to use the Vault CLI directly:

```
vault-manager env --bash
\export VAULT_ADDR=http://localhost:8200;
\export VAULT_TOKEN=$SOME_UUID;
\unset VAULT_SKIP_VERIFY;

eval $(vault-manager env --bash)
```

[vault]:  https://vaultproject.io
[spruce]: https://github.com/geofffranks/spruce
