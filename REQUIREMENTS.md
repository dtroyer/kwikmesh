# kwikmesh Requirements

This file lists what kwikmesh must do, as currently implemented. The
[README](README.md) explains how to use it. The IDs (for example R-CLI-3) are
there so that issues, commits and tests can refer to a single requirement.

## Purpose

- **R-GEN-1** Generate WireGuard configurations for a set of endpoints in
  a YAML file. Each endpoint's configuration lists the peers it shall be
  configured for.
- **R-GEN-2** Support three output formats: `wg` (a wg-quick config), `uci` (an
  OpenWRT `/etc/config/network` fragment) and `uci-script` (a shell script of
  `uci` commands).

## Network file

### Loading

- **R-FILE-1** The network configuration file is `config.yaml` unless `--config`
  or environment variable `KWIKMESH_CONFIG` is present. `--config` takes precedence
  over the environment variable.
- **R-FILE-2** Unknown YAML fields are errors.
- **R-FILE-3** Relative paths in the file, such as `private_key_file`, are
  resolved against the directory that contains the YAML file.
- **R-FILE-4** Validation doesn't stop at the first error. It reports the first
  problem in each endpoint, plus every problem with `peers` lists, preshared
  keys and defaults. Each message names the endpoint, the `defaults` section,
  or the `preshared_keys` entry it came from.
- **R-FILE-5** At least one endpoint is required.

### Defaults

- **R-DEF-1** These fields can be set under `defaults`: `interface`, `format`,
  `listen_port`, `mtu`, `dns`, `persistent_keepalive`, `pre_up`, `post_up`,
  `pre_down` and `post_down`. An endpoint that doesn't set one of them uses
  the default.
- **R-DEF-2** If neither the endpoint nor `defaults` sets `interface`, it is
  `wg0`.
- **R-DEF-3** If an endpoint sets `persistent_keepalive: 0`, that overrides a
  non-zero default.
- **R-DEF-4** For `dns` and for each hook, an endpoint value replaces the
  default; the two lists are not combined. An empty list (`[]`) on an endpoint
  overrides the default with nothing.
- **R-DEF-5** An invalid default is reported once, under `defaults:`, not once
  for every endpoint that inherits it.

### Endpoints

- **R-EP-1** Each endpoint has a unique name that starts with a letter or digit
  and contains only letters, digits, `_`, `.` and `-`.
- **R-EP-2** At least one address is required. A bare IP address is treated as
  a host prefix (/32 or /128), and the host bits of a prefix are kept.
- **R-EP-3** `listen_port` and `persistent_keepalive` must be between 0 and
  65535, and `mtu` must not be negative. A value of 0 means the setting is
  omitted from the output.
- **R-EP-4** `endpoint` is `host`, `host:port` or `[ipv6]:port`. Without a
  port, the endpoint's `listen_port` is used, and it is an error if that isn't
  set either. An endpoint without an `endpoint` is not reachable by its peers,
  and they get no `Endpoint` for it.
- **R-EP-5** `allowed_ips` are the prefixes that every other peer routes to
  this endpoint. They are normalized to their network address. If unset, they
  are the host prefixes of the endpoint's addresses.
- **R-EP-6** `dns` entries must not be empty.
- **R-EP-7** `description` is optional free text, shown as a comment on the
  peer in wg output.

### Keys

- **R-KEY-1** All keys are base64-encoded 32-byte Curve25519 keys.
- **R-KEY-2** An endpoint needs only a public key to appear as a peer. A
  private key, given inline (`private_key`) or in a file (`private_key_file`),
  is needed only to generate that endpoint's own configuration.
- **R-KEY-3** `private_key` and `private_key_file` are mutually exclusive.
- **R-KEY-4** When a private key is available, the public key is derived from
  it. A `public_key` given as well must match the derived key.
- **R-KEY-5** A key file is read only when its key is needed: to derive a
  missing public key, or to generate that endpoint's configuration. The file
  may therefore be missing on machines that never generate that endpoint.

### Peering

- **R-PEER-1** By default every endpoint peers with every other endpoint.
- **R-PEER-2** An endpoint's `peers` list restricts whom it peers with. Every
  name in the list must be a known endpoint other than itself.
- **R-PEER-3** Peering is symmetric: A and B are peers only if each allows the
  other.
- **R-PEER-4** Peers appear in the output in the order they appear in the
  file.
- **R-PEER-5** Building an endpoint's configuration fails if two of its peers
  claim the same allowed IP, because WireGuard would silently route it to only
  one of them.
- **R-PEER-6** `persistent_keepalive` set on an endpoint applies to every peer
  in that endpoint's own configuration.

### Preshared keys

- **R-PSK-1** Each `preshared_keys` entry names exactly two different, known
  endpoints and a valid key.
- **R-PSK-2** Each pair of endpoints has at most one preshared key, whatever
  order the two are listed in.
- **R-PSK-3** The key is used on both sides of the pair.

### Hooks

- **R-HOOK-1** `pre_up`, `post_up`, `pre_down` and `post_down` can each be one
  command or a list of commands.
- **R-HOOK-2** Each command must be non-empty and fit on one line, because
  wg-quick reads one command per line. Surrounding whitespace is removed.
- **R-HOOK-3** Commands are written exactly as given, so wg-quick's `%i`
  substitution works.
- **R-HOOK-4** Hooks are shown as comments in the `uci` and `uci-script`
  output formats as OpenWRT does not support them.

### Output format selection

- **R-FMT-1** An endpoint's `format` field is `wg`, `uci` or `uci-script`.
  Unknown names are errors in `generate`, `list` and `validate`.
- **R-FMT-2** Without `--format`, an endpoint is generated in its own format,
  else the format under `defaults`, else `wg`.
- **R-FMT-3** `--format` overrides the format of every endpoint generated.

## Output formats

### All formats

- **R-OUT-1** Each output starts with a comment that names the endpoint, says
  kwikmesh generated it, and explains how to install it.
- **R-OUT-2** `generate --interface` overrides the interface name in the output.

### wg

- **R-WG-1** The output is a wg-quick config with one `[Interface]` section and
  one `[Peer]` section per peer.
- **R-WG-2** `[Interface]` contains `PrivateKey` and `Address`, then
  `ListenPort`, `MTU` and `DNS` when they are set, then the hooks in the order
  `PreUp`, `PostUp`, `PreDown`, `PostDown`, one line per command.
- **R-WG-3** Each `[Peer]` contains the peer's name (and description, collapsed
  to one line) as comments, then `PublicKey`, `PresharedKey` if any,
  `Endpoint` if any (IPv6 hosts in brackets), `AllowedIPs`, and
  `PersistentKeepalive` if non-zero.

### uci and uci-script

- **R-UCI-1** The interface and peer section names contain only letters, digits
  and `_`; any other character becomes `_`. Each peer's section is named
  `<interface>_<peer>`, and its type is `wireguard_<interface>`.
- **R-UCI-2** Every value is single-quoted so that it is safe both in a UCI
  config file and in `sh`.
- **R-UCI-3** Each peer has `route_allowed_ips` set to `1` and its name as its
  `description`.
- **R-UCI-4** OpenWRT has no equivalent of wg-quick hooks, so they are listed as
  comments saying they were not applied. They are never silently dropped.
- **R-UCI-5** The `uci-script` output starts with `set -e`, deletes the existing
  interface and all of its peers, recreates them, runs `uci commit network`,
  and reloads the network. Running it twice has the same result as running it
  once.

## Command line

- **R-CLI-1** Options are GNU-style: long options take two dashes, and the
  common ones also have single-letter short forms:

  | Long          | Short |
  |---------------|-------|
  | `--config`    | `-c`  |
  | `--format`    | `-f`  |
  | `--output`    | `-o`  |
  | `--interface` | `-i`  |
  | `--all`       | `-a`  |
  | `--dir`       | `-d`  |

- **R-CLI-2** Options may come before or after the endpoint name. `--config`
  may also come after the command.
- **R-CLI-3** The commands are `generate` (alias `gen`), `list` (alias `ls`),
  `validate`, `genkey`, `genpsk` and `pubkey`. `--help` works on the program
  and on each command.
- **R-CLI-4** Every error is printed to stderr prefixed with `kwikmesh:`, and the
  exit status is 1. This includes an unknown command and a missing command.

### generate

- **R-CLI-5** `generate <endpoint>` writes one endpoint's configuration to
  stdout, or to `--output`. It takes exactly one endpoint name and at most one
  `--format`.
- **R-CLI-6** `generate --all --dir <dir>` writes every endpoint's
  configuration into the directory, creating it if needed, and prints a
  `wrote <path>` line for each file. With `--all`, `--format` can name several
  formats, comma-separated or repeated, and each endpoint is written in each
  of them.
- **R-CLI-7** File names are `<endpoint>.conf` for wg, `<endpoint>.uci` for
  uci and `<endpoint>.sh` for uci-script.
- **R-CLI-8** `--all` cannot be combined with an endpoint name or with
  `--output`, and `--dir` is only valid with `--all`.
- **R-CLI-9** With `--all`, a failure for one endpoint doesn't stop the others.
  All failures are reported at the end, and the exit status is 1.

### Other commands

- **R-CLI-10** `list` prints a table with the columns NAME, FORMAT, ADDRESSES,
  ENDPOINT and PEERS. FORMAT is the format the endpoint would be generated in
  without `--format`.
- **R-CLI-11** `validate` loads and checks the network file (including format
  names) and prints `<file>: OK` on success.
- **R-CLI-12** `genkey`, `genpsk` and `pubkey` behave like `wg genkey`,
  `wg genpsk` and `wg pubkey`. `pubkey` reads one private key from stdin, with
  or without a trailing newline. Generated private keys are clamped as
  Curve25519 requires.

## Security

- **R-SEC-1** Generated files may contain private keys, so they are created with
  mode 0600. `uci-script` files are created with mode 0700 so they can be
  executed. The `--all` output directory is created with mode 0700.
- **R-SEC-2** Files are written to a temporary file in the same directory and
  then renamed into place, so a failed write never leaves a partial file and
  the file is never briefly readable with looser permissions.

## Build and development

- **R-DEV-1** kwikmesh is written in Go, and needs Go 1.23 or later.
- **R-DEV-2** The only dependencies are `gopkg.in/yaml.v3` for the network
  file and `github.com/urfave/cli/v3` for the command line.
- **R-DEV-3** `make` runs lint, tests and the build. Lint fails on any file
  that isn't gofmt-formatted or that `go vet` rejects, and also runs
  `golangci-lint` if it is installed.
- **R-DEV-4** The program is split into `internal/network` (loading,
  validation and building each endpoint's configuration), `internal/render`
  (the output formats) and `cmd/kwikmesh` (the command line).
  `internal/network` must not import `internal/render`, which is why the CLI
  checks format names.
- **R-DEV-5** Each package has tests, and the command line is tested through
  `commands.RunCommands` without starting a process.

## Known limitations

- urfave/cli still accepts single-dash long options such as `-format`, and its
  message for an unknown option shows one dash (`-nope`) even when two were
  typed.
- Hooks are not applied in OpenWRT output (R-UCI-4).
