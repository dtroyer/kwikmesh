# kwikmesh

Manage Wireguard configurations for mesh nodes in a single YAML file.
The configuration for each endpoint includes both the Interface and Peer
information for that node as well as some additional fields describing
e.g. which peers are to be configured for that endpoint.

The output formats include the standard `wg-quick` INI-style as well
as OpenWRT's UCI config file format.

## Building

kwikmesh needs Go 1.23 or later. `make` runs lint, the tests and the
build, and leaves the `km` binary in `bin/`.

## The network file

The network is described in one YAML file, `config.yaml` by default. Use
`--config` (`-c`) or the `KWIKMESH_CONFIG` environment variable to name
another one; `--config` wins if both are set.

```yaml
defaults:                  # optional; applies to every endpoint
  interface: wg0           # the default is wg0
  format: wg               # wg, uci or uci-script; the default is wg
  listen_port: 51820
  mtu: 1420
  dns: [10.0.0.1]
  persistent_keepalive: 25
  post_up: iptables -A FORWARD -i %i -j ACCEPT    # one command or a list
  post_down: iptables -D FORWARD -i %i -j ACCEPT

endpoints:
  - name: router
    description: Home router
    format: uci-script               # this one runs OpenWRT
    addresses: [10.0.0.1/24, "fd00::1/64"]
    endpoint: vpn.example.com        # port defaults to listen_port
    allowed_ips: [10.0.0.0/24, 192.168.1.0/24]
    private_key_file: keys/router.key   # relative to this file
    persistent_keepalive: 0          # overrides the default

  - name: laptop
    addresses: [10.0.0.2]            # a bare address is a /32 or /128
    endpoint: "[2001:db8::2]:51821"
    private_key: <base64 private key>

  - name: phone
    addresses: [10.0.0.3]
    public_key: <base64 public key>  # enough to be a peer
    peers: [router]                  # only peer with these

preshared_keys:
  - endpoints: [laptop, router]
    key: <base64 preshared key>
```

- Every endpoint peers with every other endpoint, unless either one has a
  `peers` list that leaves the other out.
- An endpoint's `allowed_ips` are what its peers route to it. They default
  to its addresses as host prefixes.
- An endpoint needs only a public key to appear as a peer. Its private key
  is needed only to generate its own configuration, so a
  `private_key_file` may be missing on machines that never do that.
- Setting `dns` or a hook on an endpoint replaces the default; `[]` clears
  it.

See [REQUIREMENTS.md](REQUIREMENTS.md) for the full rules.

## Usage

```sh
km validate                      # check the network file
km list                          # list endpoints, formats and peers
km generate laptop               # print laptop's configuration
km gen laptop -o /etc/wireguard/wg0.conf
km gen router -f wg -i wg1       # override the format and interface
km gen --all --dir out           # every endpoint, in its own format
km gen -a -d out -f wg,uci       # every endpoint, in both formats
```

`generate --all` writes `<endpoint>.conf` (wg), `<endpoint>.uci` (uci) or
`<endpoint>.sh` (uci-script) into the directory. Files are created with
mode 0600, or 0700 for scripts, because they contain private keys. An
endpoint whose private key isn't available, such as `phone` above, is
skipped with a `skipped <endpoint>: no private key` line, so its key can be
added later.

Keys are compatible with `wg`:

```sh
km genkey > router.key
km pubkey < router.key
km genpsk
```

### Output formats

- `wg` is a wg-quick configuration for `/etc/wireguard/<interface>.conf`.
- `uci` is a fragment of OpenWRT's `/etc/config/network`.
- `uci-script` is a shell script of `uci` commands that replaces the
  interface and its peers, commits and reloads the network. It is safe to
  run more than once.

OpenWRT has no equivalent of wg-quick's `PreUp`, `PostUp`, `PreDown` and
`PostDown` hooks, so the OpenWRT formats list them as comments instead.

## License

Much of kwikmesh was written with the help of an AI coding assistant, and
some of it may not be protected by copyright. Every part of kwikmesh that is
protected by copyright is licensed under the MIT License; see
[LICENSE](LICENSE).

## Notes

This is the third iteration of this project, primarily built from the
`REQUIREMENTS.md` generated from earlier versions to see how refining the
steps and starting over changes the ultimate output.  I have not preserved
the earlier versions as the goal was to get requirements built that captured
what actually went in to the project.

I am not (yet?) sure any conclusions can be immediately reached other than
each fresh start clarified requirements.  Finally writing something once and
throwing it away isn't as painful as it once was.
