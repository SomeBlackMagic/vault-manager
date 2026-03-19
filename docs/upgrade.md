# safe CLI — Upgrade Plan

## Overview

This document describes a phased upgrade plan for the `vault-manager` CLI application. The current state:

- **Language**: Go 1.14, module `github.com/SomeBlackMagic/vault-manager`
- **Structure**: Monolithic — `main.go` (4,484 lines, 45 commands), `vault/vault.go` (1,151 lines)
- **Tests**: Only `vault/utils_test.go` (125 lines). Bash integration tests (3,407 lines) in `tests` file
- **Dependencies**: Outdated (2016–2020 era), 20 deprecated `ioutil` calls, deprecated `ssh/terminal`, `x509.ParseCRL`

## Dependency Graph

```
Phase 1 (Tests) → Phase 2 (Split) → Phase 3 (Go Upgrade) → Phase 4 (Package Rename) → Phase 5 (Sync Feature)
                                                                                              ↓
                                                                                        Phase 6 (Completion)
```

---

## Phase 1: Unit Tests for Base Functionality

### Goal
Establish unit test coverage for all core packages before any refactoring begins. Tests serve as a safety net for Phases 2–5.

### Deliverables

| File to Create | Tests For | Key Test Cases |
|----------------|-----------|----------------|
| `vault/secret_test.go` | `vault/secret.go` | `NewSecret`, `Has`/`Get`/`Set`/`Delete`/`Keys`/`Empty`, `JSON`/`YAML` output, `MarshalJSON`/`UnmarshalJSON` roundtrip, `SingleValue`, `Format` (crypt-md5/sha256/sha512/bcrypt/base64 — verify output prefix format), `Password` (length, policy) |
| `vault/errors_test.go` | `vault/errors.go` | `NewSecretNotFoundError`, `NewKeyNotFoundError`, `IsNotFound`/`IsSecretNotFound`/`IsKeyNotFound` for correct and incorrect error types |
| `vault/utils_test.go` (**extend**) | `vault/utils.go` | Add: `EscapePathSegment`, `EncodePath`, `PathHasKey`, `PathHasVersion`, `Canonicalize` |
| `vault/tree_test.go` | `vault/tree.go` | `PathLessThan`, `Secrets.Sort`, `Secrets.Merge`, `Secrets.Paths`, `SecretEntry.Basename` |
| `rc/rc_suite_test.go` | — | Ginkgo bootstrap for `rc` package |
| `rc/config_test.go` | `rc/config.go` | `Config.SetTarget`, `SetCurrent`, `SetToken`, `Find` (by alias/URL), `Vault`, `URL`, `Apply` (sets env vars). Use temp `$HOME` for file I/O |
| `main_suite_test.go` | — | Ginkgo bootstrap for `package main` |
| `runner_test.go` | `runner.go` | `NewRunner`, `Dispatch` (registers handler + help), `Execute` (correct handler, unknown command error), `Help` output |
| `util_test.go` | `util.go` | `duration("10h"/"5d"/"3m"/"2y"/invalid)`, `uniq` (dedup, empty, single) |
| `names_test.go` | `names.go` | `RandomName` returns `adjective-noun` format, non-empty |
| `ui_test.go` | `ui.go` | `parseKeyVal("k=v", "k=", "k@file", "k")` |

All tests use **Ginkgo/Gomega** (existing project convention).

### Verification

```bash
go test ./vault/...
go test ./rc/...
go test .
go vet ./...
make test  # integration tests still pass
```

### Estimated Effort
3–5 days

### Dependencies
None (starting phase)

---

## Phase 2: Split Monolithic Files

### Goal
Reduce `main.go` from 4,484 lines to ~250 lines. Split `vault/vault.go` (1,151 lines) into domain-specific files. Improve code navigation and maintainability.

### Strategy: Same-Package Split

Create `cmd_*.go` files in the project root, all in `package main`. This avoids circular imports and keeps access to unexported UI helpers (`pr()`, `parseKeyVal()`, `fail()`, `warn()`).

Each file exports a registration function: `registerXxxCommands(r *Runner, opt *Options)`.

### Deliverables — Command Files

| File to Create | Commands | Approx Lines |
|----------------|----------|-------------|
| `cmd_help.go` | `help`, `version`, `envvars`, `commands` | ~70 |
| `cmd_targets.go` | `target`, `targets`, `target delete`, `env` | ~360 |
| `cmd_auth.go` | `auth`, `logout`, `renew` | ~260 |
| `cmd_secrets.go` | `get`, `set`, `ask`, `paste`, `delete`, `undelete`, `revert`, `exists` + `writeHelper` | ~700 |
| `cmd_tree.go` | `ls`, `tree`, `paths`, `versions` | ~180 |
| `cmd_migration.go` | `export`, `import`, `copy`, `move` | ~400 |
| `cmd_generate.go` | `gen`, `uuid`, `ssh`, `rsa`, `dhparam` | ~340 |
| `cmd_utils.go` | `fmt`, `prompt`, `option`, `vault`, `curl` | ~280 |
| `cmd_x509.go` | `x509 validate`/`issue`/`reissue`/`renew`/`revoke`/`show`/`crl` | ~1000 |
| `cmd_admin.go` | `init`, `unseal`, `seal`, `status`, `rekey`, `local` | ~660 |

### Resulting `main.go` (~250 lines)

Retains only:
- `Options` struct (CLI flag definitions)
- `connect()`, `getVaultURL()` helpers
- Slim `main()` that creates Runner, calls registration functions, parses CLI args, executes

```go
func main() {
    var opt Options
    // ... defaults ...
    go Signals()
    r := NewRunner()
    registerHelpCommands(r, &opt)
    registerTargetCommands(r, &opt)
    registerAdminCommands(r, &opt)
    registerAuthCommands(r, &opt)
    registerSecretCommands(r, &opt)
    registerTreeCommands(r, &opt)
    registerMigrationCommands(r, &opt)
    registerGenerateCommands(r, &opt)
    registerUtilCommands(r, &opt)
    registerX509Commands(r, &opt)
    // ... CLI parsing loop ...
}
```

### Deliverables — Vault Package Split

| File to Create | Content | Approx Lines |
|----------------|---------|-------------|
| `vault/vault.go` (keep) | `Vault` struct, `NewVault`, `Client`, `MountVersion`, `Versions`, `Curl`, `Mounts`, `IsMounted`, `Mount`, `SetURL` | ~500 |
| `vault/read_write.go` | `Read`, `Write`, `List` | ~80 |
| `vault/delete.go` | `Delete`, `DeleteTree`, `DeleteVersions`, `DestroyVersions`, `Undelete` + internal helpers | ~340 |
| `vault/copy_move.go` | `Copy`, `Move`, `MoveCopyTree` | ~220 |
| `vault/pki.go` | `CertOptions`, `CreateSignedCertificate`, `RevokeCertificate`, `CheckPKIBackend`, `RetrievePem`, `FindSigningCA`, `SaveSealKeys` | ~200 |

### Verification

```bash
go build .
go vet ./...
go test ./...
make test
wc -l main.go  # expect ~250
```

### Estimated Effort
3–5 days

### Dependencies
Phase 1 must be complete (tests verify no regressions)

---

## Phase 3: Upgrade Go Version

### Goal
Update from Go 1.14 to Go 1.22+. Fix all deprecations. Update dependencies and CI.

### 3.1 Module Update

`go.mod` line 3: `go 1.14` → `go 1.22`

### 3.2 Deprecation Fixes (25 occurrences across 12 files)

| Deprecation | Occurrences | Files | Replacement |
|-------------|-------------|-------|-------------|
| `io/ioutil` (Go 1.16) | 20 | `main.go`, `ui.go`, `rc/config.go`, `vault/vault.go`, `vault/proxy.go`, `vault/strongbox.go` | `os.ReadFile`, `io.ReadAll`, `os.WriteFile`, `os.CreateTemp` |
| `math/rand.Seed` (Go 1.20) | 1 | `names.go:54` | Remove — auto-seeded in Go 1.20+ |
| `x/crypto/ssh/terminal` | 3 | `sig.go`, `prompt/prompt.go`, `vault/rekey.go` | `golang.org/x/term` (`terminal.ReadPassword` → `term.ReadPassword`) |
| `x509.ParseCRL` (Go 1.19) | 1 | `vault/x509.go:130` | `x509.ParseRevocationList` |

### 3.3 Dependency Updates

```bash
go get -u github.com/cloudfoundry-community/vaultkv@latest
go get -u golang.org/x/crypto@latest
go get -u golang.org/x/net@latest
go get -u github.com/onsi/ginkgo/v2@latest   # v1 → v2 migration
go get -u github.com/onsi/gomega@latest
go get -u github.com/mattn/go-isatty@latest
go get -u gopkg.in/yaml.v2@latest
go mod tidy
```

**Ginkgo v1 → v2**: Update import path in ALL `*_test.go` files from `github.com/onsi/ginkgo` to `github.com/onsi/ginkgo/v2`.

### 3.4 Makefile Update

```makefile
# Replace deprecated `go get` for tool install:
release: build
    mkdir -p $(RELEASE_ROOT)
    @go install github.com/mitchellh/gox@latest
    gox -osarch="$(TARGETS)" --output="..." $(GO_LDFLAGS)

# Add unit-test target:
unit-test:
    go test ./...

test: build unit-test
    ./tests
```

### 3.5 CI Pipeline Update

`ci/pipeline.yml`: Update Go version from `1.16` to `1.22`.

### Verification

```bash
go build .
go vet ./...
go test ./...
make test
```

### Estimated Effort
2–3 days

### Dependencies
Phase 2 must be complete (split files make deprecation fixes easier to locate)

---

## Phase 4: Rename Package

### Goal
Rename Go module from `github.com/SomeBlackMagic/vault-manager` to `github.com/SomeBlackMagic/vault-manager`.

### 4.1 Files to Modify

**`go.mod`** (line 1):
```
- module github.com/SomeBlackMagic/vault-manager
+ module github.com/SomeBlackMagic/vault-manager
```

**Go source files** — update all internal imports (6 occurrences in 5 files):

| File | Old Import | New Import |
|------|-----------|------------|
| `main.go:36` | `github.com/SomeBlackMagic/vault-manager/prompt` | `github.com/SomeBlackMagic/vault-manager/prompt` |
| `main.go:37` | `github.com/SomeBlackMagic/vault-manager/rc` | `github.com/SomeBlackMagic/vault-manager/rc` |
| `main.go:38` | `github.com/SomeBlackMagic/vault-manager/vault` | `github.com/SomeBlackMagic/vault-manager/vault` |
| `ui.go:12` | `github.com/SomeBlackMagic/vault-manager/prompt` | `github.com/SomeBlackMagic/vault-manager/prompt` |
| `vault/rekey.go:11` | `github.com/SomeBlackMagic/vault-manager/prompt` | `github.com/SomeBlackMagic/vault-manager/prompt` |
| `vault/utils_test.go:6` | `github.com/SomeBlackMagic/vault-manager/vault` | `github.com/SomeBlackMagic/vault-manager/vault` |

**New test files from Phase 1** will also need the new module path (write them with the new path if Phase 4 is done first, or update after).

**Non-Go files**: No references to `github.com/SomeBlackMagic/vault-manager` found in README.md, Makefile, CI configs, or other non-Go files.

### 4.2 Step-by-Step Process

1. Update `module` directive in `go.mod`
2. Find-and-replace `github.com/SomeBlackMagic/vault-manager` → `github.com/SomeBlackMagic/vault-manager` across all `.go` files
3. Run `go mod tidy`
4. Verify build: `go build .`
5. Verify tests: `go test ./...`

### Verification

```bash
go build .
go vet ./...
go test ./...
grep -r "starkandwayne/safe" --include="*.go" .  # expect 0 results
```

### Estimated Effort
0.5–1 day

### Dependencies
Phase 3 must be complete (all imports are already updated for Go 1.22)

---

## Phase 5: Filesystem-Based KV Secret Management

### Goal
Add Terraform-style `pull`/`plan`/`apply` workflow for managing Vault KV secrets through local filesystem. Secrets are stored as JSON files mirroring the Vault path structure.

### 5.1 New Package: `sync/`

```
sync/
├── types.go       — ChangeType, Change, ChangeSet, LocalSecret, VaultAccessor interface
├── state.go       — ReadLocalState, WriteLocalSecret (filesystem I/O)
├── diff.go        — ComputeChanges, FormatDiff, FormatChangeSummary
├── pull.go        — Pull (download from Vault to local JSON files)
├── plan.go        — Plan (compare local state vs Vault state)
├── apply.go       — Apply (push local changes to Vault)
└── sync_test.go   — Unit tests (mock VaultAccessor, temp directories)
```

### 5.2 Core Types (`sync/types.go`)

```go
package sync

type ChangeType int

const (
    ChangeNone   ChangeType = iota  // identical — no action
    ChangeAdd                        // local only → create in Vault
    ChangeModify                     // both exist, values differ → update Vault
    ChangeDelete                     // Vault only → delete from Vault
)

type Change struct {
    Type       ChangeType
    Path       string
    LocalData  map[string]string  // nil if Vault-only
    RemoteData map[string]string  // nil if local-only
}

type ChangeSet struct {
    Changes []Change
}

type LocalSecret struct {
    Path string
    Data map[string]string
}

// VaultAccessor abstracts Vault operations for testability
type VaultAccessor interface {
    Read(path string) (*vault.Secret, error)
    Write(path string, s *vault.Secret) error
    Delete(path string, opts vault.DeleteOpts) error
    List(path string) ([]string, error)
    ConstructSecrets(path string, opts vault.TreeOpts) (vault.Secrets, error)
}
```

### 5.3 JSON File Format

Each Vault secret maps to `<local-dir>/<vault-path>.json`:

```
local-dir/
  secret/
    app/
      db.json           ← {"username": "admin", "password": "s3cret"}
      cache.json        ← {"host": "redis:6379", "password": "xxx"}
    shared/
      tls.json          ← {"certificate": "...", "key": "..."}
```

### 5.4 New Commands

#### `vault-manager sync pull <vault-path> <local-dir>`

Downloads secrets from Vault to local JSON files.

1. Fetch all secret paths via `v.ConstructSecrets(vaultPath, TreeOpts{FetchKeys: true})`
2. For each path: `v.Read(path)` to get secret data
3. If local file exists and differs from Vault: show colored diff, prompt user:
   - `(l)` Keep local
   - `(r)` Keep remote
   - `(s)` Skip
4. Write JSON file or skip based on user choice

#### `vault-manager sync plan <vault-path> <local-dir>`

Shows what would change (like `terraform plan`).

1. Read local state: walk `<local-dir>`, parse all `.json` files
2. Read remote state from Vault
3. Compute and display ChangeSet:
   - `@G{+ secret/app/new}` — exists locally, not in Vault (will be created)
   - `@Y{~ secret/app/db}` — exists in both, values differ (will be updated) + show key-level diff
   - `@R{- secret/app/old}` — exists in Vault, not locally (will be deleted)
   - `  secret/app/same` — identical (no change)
4. Summary line: `"Plan: 3 to add, 1 to change, 2 to destroy."`

#### `vault-manager sync apply <vault-path> <local-dir>`

Applies local state to Vault (destructive).

1. Run plan (reuse plan logic), display output
2. Prompt: `"Do you want to apply these changes? (yes/no)"`
3. On confirmation:
   - `ChangeAdd` / `ChangeModify`: `v.Write(path, secret)`
   - `ChangeDelete`: `v.Delete(path, DeleteOpts{})`
4. Report: `"Apply complete! 3 added, 1 changed, 2 destroyed."`

### 5.5 CLI Registration

New file `cmd_sync.go` (package main):

```go
func registerSyncCommands(r *Runner, opt *Options) {
    r.Dispatch("sync pull", ...)   // NonDestructiveCommand
    r.Dispatch("sync plan", ...)   // NonDestructiveCommand
    r.Dispatch("sync apply", ...)  // DestructiveCommand
}
```

Add to `Options` struct in `main.go`:

```go
Sync struct {
    Pull  struct{} `cli:"pull"`
    Plan  struct{} `cli:"plan"`
    Apply struct{} `cli:"apply"`
} `cli:"sync"`
```

### 5.6 Edge Cases

- Empty Vault path → list from root
- Missing local directory on pull → create with `os.MkdirAll`
- JSON with non-string values → reject with clear error message
- Vault paths with special characters (`:`, `^`) → use `vault.EscapePathSegment()`
- Empty local directory on plan → all remote secrets shown as deletes

### 5.7 Test Plan (`sync/sync_test.go`)

| Test | Description |
|------|------------|
| `ReadLocalState` | Create temp dir with JSON files, verify correct parsing |
| `WriteLocalSecret` | Write to temp dir, verify file content and directory structure |
| `ComputeChanges` | Test all four `ChangeType` values with mock data |
| `FormatDiff` | Verify colored diff output format |
| `FormatChangeSummary` | Verify summary string |
| `Pull` | Mock `VaultAccessor`, verify correct file writes |
| `Plan` | Mock `VaultAccessor`, verify `ChangeSet` computation |
| `Apply` | Mock `VaultAccessor`, verify correct `Write`/`Delete` calls |

### Verification

```bash
go build .
go test ./sync/...
go test ./...
# Manual testing with a Vault instance:
vault-manager sync pull secret/ ./local-secrets
vault-manager sync plan secret/ ./local-secrets
vault-manager sync apply secret/ ./local-secrets
```

### Estimated Effort
5–8 days

### Dependencies
Phase 4 must be complete (new package uses renamed module path)

---

## Phase 6: Shell Completion (bash, zsh, fish)

### Goal

Add shell autocompletion support for all 47+ commands, subcommands, and flags. Currently the project has zero completion support, and the CLI framework (`github.com/jhunt/go-cli` from 2017) provides no built-in completion generation.

### Strategy: Custom Generator

Build a custom `completion/` package that:
- Extracts command names from `Runner.Topics` and flags from `Options` struct tags via reflection
- Generates static completion scripts for **bash**, **zsh**, and **fish**
- Exposes via `vault-manager completion <shell>` command (outputs script to stdout)

This avoids migrating to Cobra (which would touch the entire codebase) while providing full completion support.

### 6.1 New Package: `completion/`

```
completion/
├── completion.go      — CommandInfo, FlagInfo types, ExtractCommands()
├── bash.go            — GenerateBash(commands) → string
├── zsh.go             — GenerateZsh(commands) → string
├── fish.go            — GenerateFish(commands) → string
└── completion_test.go — Unit tests
```

### 6.2 Core Types (`completion/completion.go`)

```go
package completion

// CommandInfo describes a registered CLI command
type CommandInfo struct {
    Name        string      // e.g. "get", "x509 issue"
    Summary     string      // from Help.Summary
    Flags       []FlagInfo  // parsed from Options struct cli tags
    Subcommands []string    // e.g. x509 → [validate, issue, revoke, ...]
}

// FlagInfo describes a single command flag
type FlagInfo struct {
    Short       string  // e.g. "-k"
    Long        string  // e.g. "--insecure"
    Description string
}

// ExtractCommands builds the command tree from Runner topics
// and Options struct (via reflection on `cli` struct tags)
func ExtractCommands(topics map[string]*Help, optType reflect.Type) []CommandInfo
```

### 6.3 Bash Completion (`completion/bash.go`)

Generates a `_safe_completions()` function using `complete -F`:

```bash
_safe_completions() {
    local cur prev commands
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="ask auth copy curl delete dhparam env exists export fmt gen get help import init local logout ls move option paste paths prompt rekey renew revert rsa seal set ssh status sync target targets tree undelete unseal uuid vault version versions x509"

    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
        return
    fi

    case "${prev}" in
        x509)   COMPREPLY=($(compgen -W "validate issue reissue renew revoke show crl" -- "${cur}")) ;;
        sync)   COMPREPLY=($(compgen -W "pull plan apply" -- "${cur}")) ;;
        target) COMPREPLY=($(compgen -W "delete" -- "${cur}")) ;;
    esac

    case "${COMP_WORDS[1]}" in
        get|read|cat) COMPREPLY=($(compgen -W "--keys --yaml" -- "${cur}")) ;;
        # ... per-command flags ...
    esac
}
complete -F _vault_manager_completions vault-manager
```

### 6.4 Zsh Completion (`completion/zsh.go`)

Generates `_vault_manager` function using zsh's `_arguments` / `_describe`:

```zsh
#compdef vault-manager
_vault_manager() {
    local -a commands
    commands=(
        'ask:Create or update a secret (prompt visible)'
        'auth:Authenticate to a Vault'
        # ... all commands with descriptions ...
    )
    _arguments -C \
        '-k[Skip TLS verification]' \
        '--insecure[Skip TLS verification]' \
        '-T[Target a specific Vault]:target:' \
        '1:command:->cmd' '*::arg:->args'

    case $state in
        cmd) _describe 'vault-manager commands' commands ;;
        args)
            case $words[1] in
                x509)  _describe 'x509 subcommands' x509cmds ;;
                sync)  _describe 'sync subcommands' synccmds ;;
                # ... per-command flags ...
            esac ;;
    esac
}
_vault_manager "$@"
```

### 6.5 Fish Completion (`completion/fish.go`)

Generates `complete -c vault-manager` statements:

```fish
complete -c vault-manager -n '__fish_use_subcommand' -a 'get' -d 'Retrieve a secret'
complete -c vault-manager -n '__fish_use_subcommand' -a 'set' -d 'Create or update a secret'
# ... all commands ...
complete -c vault-manager -s k -l insecure -d 'Skip TLS verification'
complete -c vault-manager -n '__fish_seen_subcommand_from x509' -a 'issue' -d 'Issue certificate'
complete -c vault-manager -n '__fish_seen_subcommand_from sync' -a 'plan' -d 'Show planned changes'
# ... subcommands and per-command flags ...
```

### 6.6 Dynamic Completions (Phase 6b — optional)

For real-time Vault path completion:
- Add hidden `vault-manager __complete <command> <partial>` command
- Reads target from `~/.saferc`, connects to Vault, calls `v.List(prefix)`
- Outputs matching paths (one per line)
- Bash/zsh/fish scripts call this for argument completion on commands like `get`, `set`, `tree`, `ls`, `delete`

Also complete target names from `~/.saferc` for `-T`/`--target` flag.

### 6.7 CLI Registration (`cmd_completion.go`)

```go
package main

func registerCompletionCommands(r *Runner, opt *Options) {
    r.Dispatch("completion", &Help{
        Summary: "Generate shell completion scripts",
        Usage:   "vault-manager completion <bash|zsh|fish>",
        Type:    AdministrativeCommand,
    }, func(command string, args ...string) error {
        cmds := completion.ExtractCommands(r.Topics, reflect.TypeOf(*opt))
        switch args[0] {
        case "bash": fmt.Print(completion.GenerateBash(cmds))
        case "zsh":  fmt.Print(completion.GenerateZsh(cmds))
        case "fish": fmt.Print(completion.GenerateFish(cmds))
        }
        return nil
    })

    // Hidden command for dynamic completions (Phase 6b)
    r.Dispatch("__complete", nil, func(command string, args ...string) error { ... })
}
```

Usage:
```bash
# Bash — add to ~/.bashrc
source <(vault-manager completion bash)

# Zsh — add to ~/.zshrc
source <(vault-manager completion zsh)

# Fish — save to completions dir
vault-manager completion fish > ~/.config/fish/completions/vault-manager.fish
```

### 6.8 Files to Create/Modify

| Action | File |
|--------|------|
| Create | `completion/completion.go` — types + `ExtractCommands` |
| Create | `completion/bash.go` — `GenerateBash` |
| Create | `completion/zsh.go` — `GenerateZsh` |
| Create | `completion/fish.go` — `GenerateFish` |
| Create | `completion/completion_test.go` — unit tests |
| Create | `cmd_completion.go` — `registerCompletionCommands` + `__complete` |
| Modify | `main.go` — add `Completion` to `Options` struct, call `registerCompletionCommands` |
| Modify | `Makefile` — add `completions` target |

### 6.9 Makefile Integration

```makefile
completions: build
    ./vault-manager completion bash > completions/vault-manager.bash
    ./vault-manager completion zsh  > completions/_vault-manager
    ./vault-manager completion fish > completions/vault-manager.fish

release: build completions
    # include completions/ in release artifacts
```

### 6.10 Testing (`completion/completion_test.go`)

| Test | Description |
|------|------------|
| `ExtractCommands` | Mock Topics map + Options type → verify full command list |
| `GenerateBash` | Output contains `complete -F`, all command names, key flags |
| `GenerateZsh` | Output contains `#compdef vault-manager`, `_vault_manager`, command descriptions |
| `GenerateFish` | Output contains `complete -c vault-manager`, subcommand conditions |
| Syntax validation | `vault-manager completion bash \| bash -n`, `vault-manager completion zsh \| zsh -n` |

### Verification

```bash
go build .
go test ./completion/...
./vault-manager completion bash | bash -n   # syntax check
./vault-manager completion zsh  | zsh -n    # syntax check
# Manual: source completion and test TAB after "safe "
```

### Estimated Effort
3–4 days (+ 1–2 days for dynamic completions in Phase 6b)

### Dependencies
Phase 2 (split files — cleaner integration) and Phase 5 (sync commands must be included in completion list)

---

## Summary

| Phase | Description | Effort | Dependencies |
|-------|------------|--------|-------------|
| **Phase 1** | Unit tests for base functionality | 3–5 days | None |
| **Phase 2** | Split monolithic files | 3–5 days | Phase 1 |
| **Phase 3** | Upgrade Go 1.14 → 1.22 | 2–3 days | Phase 2 |
| **Phase 4** | Rename package to `github.com/SomeBlackMagic/vault-manager` | 0.5–1 day | Phase 3 |
| **Phase 5** | Filesystem-based KV sync (pull/plan/apply) | 5–8 days | Phase 4 |
| **Phase 6** | Shell completion (bash/zsh/fish) | 3–4 days | Phase 2, Phase 5 |
| **Total** | | **16.5–26 days** | |

## Dependency Graph

```
Phase 1 (Tests) → Phase 2 (Split) → Phase 3 (Go Upgrade) → Phase 4 (Rename) → Phase 5 (Sync)
                                                                                    ↓
                                                                               Phase 6 (Completion)

Phase 6 (this doc) ─────────────── can run in parallel ──────────────────────────────
```

## Architectural Decisions

1. **Same-package split** (`cmd_*.go` in `package main`) instead of a separate `cmd/` package — avoids circular imports with `Runner`/`Handler` types and avoids exporting UI helpers
2. **`VaultAccessor` interface** in `sync/` package — enables unit testing without a live Vault server
3. **Ginkgo v2 migration** — v1 is unmaintained; Go 1.22 has better compatibility with v2
4. **JSON format** for local sync files — easy to diff, wide tooling support, `vault.Secret` already has `MarshalJSON`/`UnmarshalJSON`
5. **Module rename** as a separate phase after Go upgrade — minimizes risk by isolating import path changes from dependency updates
6. **Custom completion generator** instead of Cobra migration — `go-cli` has no completion support, Cobra migration would touch the entire codebase. Custom generator uses reflection on `Options` struct `cli` tags + `Runner.Topics` map

## Critical Files Reference

| File | Role | Affected Phases |
|------|------|----------------|
| `main.go` (4,484 lines) | All 45 commands, Options struct, `connect()` | 1, 2, 4, 5, 6 |
| `vault/vault.go` (1,151 lines) | Vault client CRUD, mounts, PKI | 1, 2, 3 |
| `vault/secret.go` (285 lines) | `Secret` struct — core data type | 1 |
| `vault/tree.go` (~500 lines) | `ConstructSecrets`, tree traversal | 1, 5 |
| `vault/x509.go` (~600 lines) | X.509 certificates, CRL | 2, 3 |
| `rc/config.go` (7,310 lines) | Configuration management | 1, 3, 6 |
| `runner.go` (124 lines) | Command dispatcher — `Topics` map used for completion | 1, 6 |
| `go.mod` | Module definition, dependencies | 3, 4 |
| `Makefile` | Build targets | 3, 6 |
| `ci/pipeline.yml` | CI/CD pipeline | 3 |
