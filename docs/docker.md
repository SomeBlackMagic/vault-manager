Running with Docker
=====================

The official image is published at `ghcr.io/someblackmagic/vault-manager`.

The container expects:
- The configuration file at `/root/.vault-managerrc` (mount from host)
- An optional working directory for `sync` operations (mount from host)

These examples run the same commands documented in
[commands.md](commands.md) and [sync.md](sync.md) — Docker is just one way
of invoking the CLI.

### One-off command

```bash
docker run --rm \
  -v "$HOME/.vault-managerrc:/root/.vault-managerrc:ro" \
  ghcr.io/someblackmagic/vault-manager \
  status
```

### Read a secret

```bash
docker run --rm \
  -v "$HOME/.vault-managerrc:/root/.vault-managerrc:ro" \
  ghcr.io/someblackmagic/vault-manager \
  get secret/myapp/db
```

### sync pull — download secrets to a local directory

```bash
docker run --rm \
  -v "$HOME/.vault-managerrc:/root/.vault-managerrc:ro" \
  -v "$(pwd)/secrets:/secrets" \
  ghcr.io/someblackmagic/vault-manager \
  sync pull secret/myapp /secrets
```

After the command completes, `./secrets/` on the host will contain the
downloaded JSON files.

### sync plan — preview changes

```bash
docker run --rm \
  -v "$HOME/.vault-managerrc:/root/.vault-managerrc:ro" \
  -v "$(pwd)/secrets:/secrets" \
  ghcr.io/someblackmagic/vault-manager \
  sync plan secret/myapp /secrets
```

### sync apply — apply changes to Vault

Because `apply` prompts for confirmation, attach a TTY with `-it`:

```bash
docker run --rm -it \
  -v "$HOME/.vault-managerrc:/root/.vault-managerrc:ro" \
  -v "$(pwd)/secrets:/secrets" \
  ghcr.io/someblackmagic/vault-manager \
  sync apply secret/myapp /secrets
```

To skip the interactive prompt in CI pipelines, pipe `y` to stdin:

```bash
echo y | docker run --rm -i \
  -v "$HOME/.vault-managerrc:/root/.vault-managerrc:ro" \
  -v "$(pwd)/secrets:/secrets" \
  ghcr.io/someblackmagic/vault-manager \
  sync apply secret/myapp /secrets
```

### Using environment variables instead of a config file

If you prefer not to mount a config file you can supply Vault connection
details via environment variables:

```bash
docker run --rm -it \
  -e VAULT_ADDR=https://vault.example.com:8200 \
  -e VAULT_TOKEN=s.xxxxxxxxxxxxxxxx \
  -v "$(pwd)/secrets:/secrets" \
  ghcr.io/someblackmagic/vault-manager \
  sync apply secret/myapp /secrets
```

Skipping TLS verification (e.g. self-signed certificates):

```bash
docker run --rm \
  -e VAULT_ADDR=https://vault.example.com:8200 \
  -e VAULT_TOKEN=s.xxxxxxxxxxxxxxxx \
  -e VAULT_SKIP_VERIFY=true \
  -v "$(pwd)/secrets:/secrets" \
  ghcr.io/someblackmagic/vault-manager \
  sync plan secret/myapp /secrets
```

### Logging

Pass the logging variables with `-e` to get diagnostic logs on the
container's stderr:

```bash
docker run --rm \
  -v "$HOME/.vault-managerrc:/root/.vault-managerrc:ro" \
  -v "$(pwd)/secrets:/secrets" \
  -e VAULT_MANAGER_LOG_LEVEL=debug \
  -e VAULT_MANAGER_LOG_FORMAT=json \
  ghcr.io/someblackmagic/vault-manager \
  sync plan secret/myapp /secrets
```

`VAULT_MANAGER_LOG_LEVEL` accepts `error`, `warn`, `info` (default),
`debug` and `trace`; `VAULT_MANAGER_LOG_FORMAT` accepts `text` (default)
and `json`. The `--log-level` / `--log-format` flags work too and take
precedence. Don't ship `trace` output to a shared CI log without
reviewing it: it lists every Vault URL that was requested.

### docker-compose example

```yaml
services:
  vault-sync:
    image: ghcr.io/someblackmagic/vault-manager
    volumes:
      - ~/.vault-managerrc:/root/.vault-managerrc:ro
      - ./secrets:/secrets
    environment:
      VAULT_ADDR: https://vault.example.com:8200
      VAULT_MANAGER_LOG_LEVEL: info
      VAULT_MANAGER_LOG_FORMAT: json
    command: sync plan secret/myapp /secrets
```

Run with:

```bash
docker compose run --rm vault-sync
```
