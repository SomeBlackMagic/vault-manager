vault-manager - A Vault CLI
==================

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

Installation
------------

### Download the latest binary

Grab the latest release for your platform from the
[Releases page](https://github.com/SomeBlackMagic/vault-manager/releases/latest).

Linux (amd64), using curl:

```bash
curl -sL https://github.com/SomeBlackMagic/vault-manager/releases/latest/download/vault-manager-linux-amd64.tar.gz | tar xz
sudo mv vault-manager-linux-amd64 /usr/local/bin/vault-manager
```

macOS (Apple Silicon), using curl:

```bash
curl -sL https://github.com/SomeBlackMagic/vault-manager/releases/latest/download/vault-manager-darwin-arm64.tar.gz | tar xz
sudo mv vault-manager-darwin-arm64 /usr/local/bin/vault-manager
```

macOS (Intel) / Linux, using wget:

```bash
wget -qO- https://github.com/SomeBlackMagic/vault-manager/releases/latest/download/vault-manager-darwin-amd64.tar.gz | tar xz
sudo mv vault-manager-darwin-amd64 /usr/local/bin/vault-manager
```

Windows (amd64), using PowerShell:

```powershell
Invoke-WebRequest -Uri "https://github.com/SomeBlackMagic/vault-manager/releases/latest/download/vault-manager-windows-amd64.zip" -OutFile "vault-manager.zip"
Expand-Archive -Path "vault-manager.zip" -DestinationPath "."
```

Then move `vault-manager-windows-amd64.exe` to a folder on your `PATH`
(renaming it to `vault-manager.exe` is recommended).

### Docker

```bash
docker run --rm \
  -v "$HOME/.vault-managerrc:/root/.vault-managerrc:ro" \
  ghcr.io/someblackmagic/vault-manager \
  status
```

See [docs/docker.md](docs/docker.md) for the full set of Docker examples
(sync, environment variables, docker-compose, etc.)

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

Documentation
--------------

- [Command Reference](docs/commands.md) — every sub-command, grouped by
  area (targets & auth, secrets, generation, listing, migration, X.509,
  admin, sync, ...)
- [Sync](docs/sync.md) — `sync pull` / `sync plan` / `sync apply` and the
  local JSON file format
- [Running with Docker](docs/docker.md) — one-off commands, sync via
  Docker, environment variables, docker-compose

[vault]:  https://vaultproject.io
[spruce]: https://github.com/geofffranks/spruce
