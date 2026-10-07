Sync — Vault ↔ local filesystem
================================

`sync` is general `vault-manager` functionality (not tied to any particular
way of running the CLI — it works the same whether you run the binary
directly or via Docker, see [docker.md](docker.md) for container examples).
It synchronizes secrets between Vault and a local directory of JSON files,
which is useful for reviewing changes before they're applied, or for
managing secrets under version control / GitOps-style workflows.

- [sync pull](#sync-pull-vault-path-local-dir)
- [sync plan](#sync-plan-vault-path-local-dir)
- [sync apply](#sync-apply-vault-path-local-dir)
- [Local JSON file format](#local-json-file-format)
- [Diagnostics](#diagnostics)

### sync pull vault-path local-dir

Download secrets from Vault to a local directory as JSON files. Each
secret path maps to a corresponding `.json` file:

```
secret/app/db  →  local-dir/secret/app/db.json
```

When a local file already exists, `pull` compares its contents against the
remote value and resolves conflicts interactively:

- **New remote secret** — written to disk automatically (shown with `+`)
- **Local == remote** — skipped, no action taken
- **Conflict** — diff is displayed and you are prompted to keep `(l)ocal`,
  `(r)emote`, or `(s)kip`; in non-TTY mode (e.g. CI pipelines) the remote
  version is kept automatically

```
vault-manager sync pull secret/myapp ./secrets
```

### sync plan vault-path local-dir

Compare local JSON files against the current Vault state and print a
field-level diff without making any changes. This is a read-only
operation.

Change indicators:

| Symbol | Meaning                                   |
|--------|-------------------------------------------|
| `+`    | Secret exists locally but not in Vault    |
| `~`    | Secret differs between local and Vault    |
| `-`    | Secret exists in Vault but not locally    |

A summary line is printed at the end:

```
Plan: 2 to add, 1 to change, 0 to destroy.
```

Example:

```
vault-manager sync plan secret/myapp ./secrets
```

### sync apply vault-path local-dir

Apply local changes to Vault. Runs the same diff as `plan`, displays it,
and then prompts for confirmation before writing anything:

```
Do you want to perform these actions? (y/n)
```

On confirmation:
- **Add** — creates secrets that exist locally but not in Vault
- **Modify** — updates secrets where local and remote values differ
- **Delete** — removes secrets that exist in Vault but not locally

Nested JSON objects stored as string values in Vault are automatically
expanded to proper JSON structures in local files (and re-serialised on
apply), so they remain human-readable on disk.

```
vault-manager sync apply secret/myapp ./secrets
```

A summary is printed after a successful run:

```
Apply complete! 2 added, 1 changed, 0 destroyed.
```

Local JSON file format
-----------------------

Each secret is stored as a pretty-printed JSON object. Simple string
values:

```json
{
  "username": "admin",
  "password": "s3cr3t"
}
```

String values that contain JSON objects or arrays are automatically
expanded into nested structures for easier editing:

```json
{
  "plain": "simple-string",
  "config": {
    "host": "db.example.com",
    "port": 5432,
    "ssl": true
  },
  "tags": ["web", "api"]
}
```

On `apply`, nested objects are re-serialised to compact JSON strings before
being written to Vault.

Diagnostics
-----------

By default `sync` prints only its regular output (diffs, prompts and the
summary). To see what it is doing, raise the log level; logs are written to
stderr:

```
vault-manager --log-level=debug sync plan secret/myapp ./secrets
```

```
level=DEBUG msg="sync plan started" vault_path=secret/myapp local_dir=./secrets
level=DEBUG msg="reading local state" local_dir=./secrets
level=DEBUG msg="read local secrets" count=3
level=DEBUG msg="fetching remote secrets" vault_path=secret/myapp
level=DEBUG msg="fetched remote secrets" count=3
level=DEBUG msg="compared secret" path=secret/myapp/db change=modify
level=DEBUG msg="sync plan completed" adds=1 changes=1 deletes=0 duration=12.4ms
```

`trace` adds a line for every Vault HTTP request, which helps with TLS,
proxy and permission problems:

```
VAULT_MANAGER_LOG_LEVEL=trace vault-manager sync pull secret/myapp ./secrets
```

For CI and log collectors, use JSON:

```
vault-manager --log-format=json --log-level=debug sync plan secret/myapp ./secrets
```

Logs contain secret **paths** and counts, never secret values, tokens or
request bodies. Note that the diff printed by `plan`/`apply`/`pull` is
regular output and does show values; that is unchanged by the log level.
Even so, avoid publishing `debug`/`trace` output in shared CI logs without
reviewing it first.
