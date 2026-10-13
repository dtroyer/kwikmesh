// internal/network/network_test.go - Tests for loading and validation

package network

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// keyReplacer replaces {privN} and {pubN} (N is 1 to 9) with test keys.
var keyReplacer = func() *strings.Replacer {
	var pairs []string
	for i := byte(1); i <= 9; i++ {
		k := testKey(i)
		pairs = append(pairs,
			fmt.Sprintf("{priv%d}", i), k.String(),
			fmt.Sprintf("{pub%d}", i), k.PublicKey().String())
	}
	return strings.NewReplacer(pairs...)
}()

// writeConfig writes config.yaml, with test keys substituted, and any
// extra files into a new directory, and returns the config's path.
func writeConfig(t *testing.T, config string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(keyReplacer.Replace(data)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(keyReplacer.Replace(config)), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func load(t *testing.T, config string) (*Network, error) {
	t.Helper()
	return Load(writeConfig(t, config, nil), Options{CheckFormat: checkFormat})
}

func mustLoad(t *testing.T, config string) *Network {
	t.Helper()
	n, err := load(t, config)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return n
}

// checkFormat stands in for the renderers' format names.
func checkFormat(name string) error {
	if name == "wg" || name == "uci" || name == "uci-script" {
		return nil
	}
	return fmt.Errorf("unknown format %q", name)
}

// errorLines returns the lines of err with the file name prefix removed.
func errorLines(t *testing.T, err error) []string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	var lines []string
	for _, l := range strings.Split(err.Error(), "\n") {
		_, rest, _ := strings.Cut(l, "config.yaml: ")
		lines = append(lines, rest)
	}
	return lines
}

func prefixes(t *testing.T, ss ...string) []netip.Prefix {
	t.Helper()
	var ps []netip.Prefix
	for _, s := range ss {
		ps = append(ps, netip.MustParsePrefix(s))
	}
	return ps
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"), Options{})
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want not exist", err)
	}
}

// R-FILE-5
func TestNoEndpoints(t *testing.T) {
	for _, config := range []string{"", "endpoints: []\n", "defaults:\n  mtu: 1400\n"} {
		_, err := load(t, config)
		want := []string{"endpoints: at least one endpoint is required"}
		if got := errorLines(t, err); !slices.Equal(got, want) {
			t.Errorf("config %q: errors = %q, want %q", config, got, want)
		}
	}
}

// R-FILE-2
func TestUnknownFields(t *testing.T) {
	_, err := load(t, `
endpoints:
  - name: a
    addresses: [10.0.0.1]
    public_key: {pub1}
    colour: blue
defaults:
  shape: round
`)
	want := []string{
		`line 6: unknown field "colour"`,
		`line 8: unknown field "shape"`,
	}
	if got := errorLines(t, err); !slices.Equal(got, want) {
		t.Errorf("errors = %q, want %q", got, want)
	}
}

// R-DEF-1, R-DEF-2, R-FMT-2: unset fields use defaults, then built-in values.
func TestDefaults(t *testing.T) {
	n := mustLoad(t, `
defaults:
  interface: wg9
  format: uci
  listen_port: 51820
  mtu: 1400
  dns: [10.0.0.53]
  persistent_keepalive: 25
  pre_up: echo pre-up
  post_up: [echo one, echo two]
  pre_down: echo pre-down
  post_down: echo post-down
endpoints:
  - name: inherits
    addresses: [10.0.0.1]
    public_key: {pub1}
  - name: overrides
    addresses: [10.0.0.2]
    public_key: {pub2}
    interface: wg1
    format: wg
    listen_port: 1000
    mtu: 1300
    dns: [1.1.1.1]
    persistent_keepalive: 10
    pre_up: echo mine
    post_up: [echo three]
    pre_down: []
    post_down: []
`)
	in := n.Endpoint("inherits")
	if in.Interface != "wg9" || in.Format != "uci" || in.ListenPort != 51820 || in.MTU != 1400 || in.PersistentKeepalive != 25 {
		t.Errorf("inherits: got %+v", in)
	}
	if !slices.Equal(in.DNS, []string{"10.0.0.53"}) ||
		!slices.Equal(in.PreUp, []string{"echo pre-up"}) ||
		!slices.Equal(in.PostUp, []string{"echo one", "echo two"}) ||
		!slices.Equal(in.PreDown, []string{"echo pre-down"}) ||
		!slices.Equal(in.PostDown, []string{"echo post-down"}) {
		t.Errorf("inherits: lists = %q %q %q %q %q", in.DNS, in.PreUp, in.PostUp, in.PreDown, in.PostDown)
	}

	// R-DEF-4: lists replace the default and are not merged; [] clears it.
	ov := n.Endpoint("overrides")
	if ov.Interface != "wg1" || ov.Format != "wg" || ov.ListenPort != 1000 || ov.MTU != 1300 || ov.PersistentKeepalive != 10 {
		t.Errorf("overrides: got %+v", ov)
	}
	if !slices.Equal(ov.DNS, []string{"1.1.1.1"}) ||
		!slices.Equal(ov.PreUp, []string{"echo mine"}) ||
		!slices.Equal(ov.PostUp, []string{"echo three"}) ||
		len(ov.PreDown) != 0 || len(ov.PostDown) != 0 {
		t.Errorf("overrides: lists = %q %q %q %q %q", ov.DNS, ov.PreUp, ov.PostUp, ov.PreDown, ov.PostDown)
	}

	n = mustLoad(t, `
endpoints:
  - name: bare
    addresses: [10.0.0.1]
    public_key: {pub1}
`)
	e := n.Endpoint("bare")
	if e.Interface != "wg0" || e.Format != "wg" { // R-DEF-2, R-FMT-2
		t.Errorf("bare: interface %q, format %q; want wg0, wg", e.Interface, e.Format)
	}
}

// R-DEF-3, R-DEF-4: zero and empty values on an endpoint override defaults.
func TestZeroOverridesDefault(t *testing.T) {
	n := mustLoad(t, `
defaults:
  persistent_keepalive: 25
  listen_port: 51820
  mtu: 1400
  dns: [10.0.0.53]
  post_up: echo up
endpoints:
  - name: a
    addresses: [10.0.0.1]
    public_key: {pub1}
    persistent_keepalive: 0
    listen_port: 0
    mtu: 0
    dns: []
    post_up: []
`)
	e := n.Endpoint("a")
	if e.PersistentKeepalive != 0 || e.ListenPort != 0 || e.MTU != 0 || len(e.DNS) != 0 || len(e.PostUp) != 0 {
		t.Errorf("got %+v, want zero values", e)
	}
}

// R-HOOK-1, R-HOOK-2, R-HOOK-3: hooks are trimmed but otherwise kept as is.
func TestHooks(t *testing.T) {
	n := mustLoad(t, `
endpoints:
  - name: a
    addresses: [10.0.0.1]
    public_key: {pub1}
    pre_up: "  iptables -A FORWARD -i %i -j ACCEPT  "
    post_down: ["  ip link set %i down", "echo 'done'"]
`)
	e := n.Endpoint("a")
	if !slices.Equal(e.PreUp, []string{"iptables -A FORWARD -i %i -j ACCEPT"}) {
		t.Errorf("PreUp = %q", e.PreUp)
	}
	if !slices.Equal(e.PostDown, []string{"ip link set %i down", "echo 'done'"}) {
		t.Errorf("PostDown = %q", e.PostDown)
	}
}

// R-EP-2, R-EP-5
func TestAddressesAndAllowedIPs(t *testing.T) {
	n := mustLoad(t, `
endpoints:
  - name: a
    addresses: [10.0.0.1/24, "fd00::1/64"]
    public_key: {pub1}
  - name: b
    addresses: [10.0.0.2, "fd00::2"]
    public_key: {pub2}
    allowed_ips: [10.0.0.2, 192.168.1.77/24, "fd00:1::5/48"]
`)
	a, b := n.Endpoint("a"), n.Endpoint("b")
	// Host bits are kept in addresses.
	if want := prefixes(t, "10.0.0.1/24", "fd00::1/64"); !slices.Equal(a.Addresses, want) {
		t.Errorf("a.Addresses = %v, want %v", a.Addresses, want)
	}
	// Default allowed IPs are the addresses' host prefixes.
	if want := prefixes(t, "10.0.0.1/32", "fd00::1/128"); !slices.Equal(a.AllowedIPs, want) {
		t.Errorf("a.AllowedIPs = %v, want %v", a.AllowedIPs, want)
	}
	// Bare addresses are host prefixes.
	if want := prefixes(t, "10.0.0.2/32", "fd00::2/128"); !slices.Equal(b.Addresses, want) {
		t.Errorf("b.Addresses = %v, want %v", b.Addresses, want)
	}
	// Allowed IPs are normalized to their network address.
	if want := prefixes(t, "10.0.0.2/32", "192.168.1.0/24", "fd00:1::/48"); !slices.Equal(b.AllowedIPs, want) {
		t.Errorf("b.AllowedIPs = %v, want %v", b.AllowedIPs, want)
	}
}

// R-EP-4
func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		in         string
		listenPort int
		host       string
		port       int
		err        string
	}{
		{in: "vpn.example.com:1234", host: "vpn.example.com", port: 1234},
		{in: "vpn.example.com", listenPort: 51820, host: "vpn.example.com", port: 51820},
		{in: "192.0.2.1:5", host: "192.0.2.1", port: 5},
		{in: "[2001:db8::1]:51821", host: "2001:db8::1", port: 51821},
		{in: "2001:db8::1", listenPort: 7, host: "2001:db8::1", port: 7},
		{in: "vpn.example.com", err: "has no port and listen_port is not set"},
		{in: "host:0", err: "port must be between 1 and 65535"},
		{in: "host:65536", err: "port must be between 1 and 65535"},
		{in: "host:http", err: "port must be between 1 and 65535"},
		{in: "[2001:db8::1]", err: "want host, host:port or [ipv6]:port"},
		{in: "2001:db8::zz", err: "want host, host:port or [ipv6]:port"},
		{in: ":51820", err: "invalid host"},
		{in: "two words", listenPort: 1, err: "invalid host"},
	}
	for _, tt := range tests {
		host, port, err := parseEndpoint(tt.in, tt.listenPort)
		if tt.err != "" {
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Errorf("parseEndpoint(%q) error = %v, want %q", tt.in, err, tt.err)
			}
			continue
		}
		if err != nil || host != tt.host || port != tt.port {
			t.Errorf("parseEndpoint(%q) = %q, %d, %v; want %q, %d", tt.in, host, port, err, tt.host, tt.port)
		}
	}
}

// R-EP-4: an endpoint without a port uses its listen_port, which may come
// from the defaults.
func TestEndpointPortFromListenPort(t *testing.T) {
	n := mustLoad(t, `
defaults:
  listen_port: 51820
endpoints:
  - name: a
    addresses: [10.0.0.1]
    public_key: {pub1}
    endpoint: a.example.com
  - name: b
    addresses: [10.0.0.2]
    public_key: {pub2}
    endpoint: b.example.com
    listen_port: 1000
  - name: c
    addresses: [10.0.0.3]
    public_key: {pub3}
`)
	for name, want := range map[string]string{"a": "a.example.com:51820", "b": "b.example.com:1000", "c": ""} {
		if got := n.Endpoint(name).EndpointAddr(); got != want {
			t.Errorf("%s: EndpointAddr() = %q, want %q", name, got, want)
		}
	}
}

// R-EP-7
func TestDescription(t *testing.T) {
	n := mustLoad(t, `
endpoints:
  - name: a
    description: The office router
    addresses: [10.0.0.1]
    public_key: {pub1}
`)
	if got := n.Endpoint("a").Description; got != "The office router" {
		t.Errorf("Description = %q", got)
	}
}

// R-FILE-4: each message names where it came from, and each endpoint
// reports only its first problem. Each entry is an endpoint that must
// produce exactly the one error shown.
func TestEndpointErrors(t *testing.T) {
	tests := []struct {
		endpoint string
		want     string
	}{
		// R-EP-1
		{"addresses: [10.0.0.9]\npublic_key: {pub9}", `endpoint #2: name is required`},
		{"name: -x\naddresses: [10.0.0.9]\npublic_key: {pub9}", `endpoint "-x": name must start with a letter or digit`},
		{"name: a b\naddresses: [10.0.0.9]\npublic_key: {pub9}", `endpoint "a b": name must start with a letter or digit`},
		{"name: ok\naddresses: [10.0.0.9]\npublic_key: {pub9}", `endpoint "ok": duplicate name`},
		// R-EP-2
		{"name: x\npublic_key: {pub9}", `endpoint "x": at least one address is required`},
		{"name: x\naddresses: [10.0.0.256]\npublic_key: {pub9}", `endpoint "x": addresses: invalid address "10.0.0.256"`},
		{"name: x\naddresses: [10.0.0.1/33]\npublic_key: {pub9}", `endpoint "x": addresses: invalid prefix "10.0.0.1/33"`},
		{"name: x\naddresses: [10.0.0.9]\nallowed_ips: [nope]\npublic_key: {pub9}", `endpoint "x": allowed_ips: invalid address "nope"`},
		// R-EP-3
		{"name: x\naddresses: [10.0.0.9]\npublic_key: {pub9}\nlisten_port: 65536", `endpoint "x": listen_port 65536: must be between 0 and 65535`},
		{"name: x\naddresses: [10.0.0.9]\npublic_key: {pub9}\nlisten_port: -1", `endpoint "x": listen_port -1: must be between 0 and 65535`},
		{"name: x\naddresses: [10.0.0.9]\npublic_key: {pub9}\npersistent_keepalive: 70000", `endpoint "x": persistent_keepalive 70000: must be between 0 and 65535`},
		{"name: x\naddresses: [10.0.0.9]\npublic_key: {pub9}\nmtu: -5", `endpoint "x": mtu -5: must not be negative`},
		// R-EP-4
		{"name: x\naddresses: [10.0.0.9]\npublic_key: {pub9}\nendpoint: host", `endpoint "x": endpoint "host" has no port and listen_port is not set`},
		// R-EP-6
		{"name: x\naddresses: [10.0.0.9]\npublic_key: {pub9}\ndns: [1.1.1.1, ' ']", `endpoint "x": dns: entries must not be empty`},
		// R-HOOK-2
		{"name: x\naddresses: [10.0.0.9]\npublic_key: {pub9}\npre_up: ' '", `endpoint "x": pre_up: commands must not be empty`},
		{"name: x\naddresses: [10.0.0.9]\npublic_key: {pub9}\npost_down: \"a\\nb\"", `endpoint "x": post_down: each command must fit on one line`},
		// R-FMT-1
		{"name: x\naddresses: [10.0.0.9]\npublic_key: {pub9}\nformat: ini", `endpoint "x": format: unknown format "ini"`},
		{"name: x\naddresses: [10.0.0.9]\npublic_key: {pub9}\ninterface: has/slash", `endpoint "x": interface: invalid interface name "has/slash"`},
		// R-KEY-*
		{"name: x\naddresses: [10.0.0.9]", `endpoint "x": public_key, private_key or private_key_file is required`},
		{"name: x\naddresses: [10.0.0.9]\npublic_key: AAAA", `endpoint "x": public_key: must be a base64-encoded 32-byte key`},
		{"name: x\naddresses: [10.0.0.9]\nprivate_key: AAAA", `endpoint "x": private_key: must be a base64-encoded 32-byte key`},
		{"name: x\naddresses: [10.0.0.9]\nprivate_key: {priv9}\nprivate_key_file: k", `endpoint "x": private_key and private_key_file are mutually exclusive`},
		{"name: x\naddresses: [10.0.0.9]\nprivate_key: {priv9}\npublic_key: {pub8}", `endpoint "x": public_key does not match the private key`},
		// Only the first problem is reported.
		{"name: x\naddresses: [bad]\nlisten_port: -1", `endpoint "x": listen_port -1: must be between 0 and 65535`},
	}
	for _, tt := range tests {
		config := "endpoints:\n  - name: ok\n    addresses: [10.0.0.1]\n    public_key: {pub1}\n  - " +
			strings.ReplaceAll(tt.endpoint, "\n", "\n    ") + "\n"
		_, err := load(t, config)
		got := errorLines(t, err)
		if len(got) != 1 || !strings.HasPrefix(got[0], tt.want) {
			t.Errorf("endpoint:\n%s\nerrors = %q, want one starting with %q", tt.endpoint, got, tt.want)
		}
	}
}

// R-FILE-4, R-DEF-5: every problem with defaults, peers lists and
// preshared keys is reported, and a bad default is reported only once.
func TestAllProblemsReported(t *testing.T) {
	_, err := load(t, `
defaults:
  listen_port: 70000
  format: ini
  mtu: -1
endpoints:
  - name: a
    addresses: [10.0.0.1]
    public_key: {pub1}
    peers: [a, nobody, b]
  - name: b
    addresses: [10.0.0.2]
    public_key: {pub2}
    endpoint: b.example.com
  - name: c
    addresses: [bad]
    mtu: -1
preshared_keys:
  - endpoints: [a]
    key: AAAA
  - endpoints: [a, a]
    key: {priv1}
  - endpoints: [a, ghost]
    key: {priv1}
  - endpoints: [a, b]
    key: {priv1}
  - endpoints: [b, a]
    key: {priv2}
`)
	want := []string{
		`defaults: format: unknown format "ini"`,
		`defaults: listen_port 70000: must be between 0 and 65535`,
		`defaults: mtu -1: must not be negative`,
		`endpoint "a": peers: an endpoint cannot peer with itself`,
		`endpoint "a": peers: unknown endpoint "nobody"`,
		`endpoint "c": mtu -1: must not be negative`,
		`preshared_keys entry 1 (a): endpoints must name exactly two endpoints`,
		`preshared_keys entry 1 (a): key: must be a base64-encoded 32-byte key`,
		`preshared_keys entry 2 (a, a): endpoints must be two different endpoints`,
		`preshared_keys entry 3 (a, ghost): unknown endpoint "ghost"`,
		`preshared_keys entry 5 (b, a): duplicate preshared key for this pair`,
	}
	if got := errorLines(t, err); !slices.Equal(got, want) {
		t.Errorf("errors:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// R-KEY-2, R-KEY-4: a public key alone is enough to be a peer, and a
// private key gives the public key.
func TestKeys(t *testing.T) {
	n := mustLoad(t, `
endpoints:
  - name: pub
    addresses: [10.0.0.1]
    public_key: {pub1}
  - name: priv
    addresses: [10.0.0.2]
    private_key: {priv2}
  - name: both
    addresses: [10.0.0.3]
    private_key: {priv3}
    public_key: {pub3}
`)
	for name, b := range map[string]byte{"pub": 1, "priv": 2, "both": 3} {
		if got, want := n.Endpoint(name).PublicKey, testKey(b).PublicKey(); got != want {
			t.Errorf("%s: PublicKey = %s, want %s", name, got, want)
		}
	}
	if _, err := n.Build("pub"); !errors.Is(err, ErrNoPrivateKey) || !strings.Contains(err.Error(), `endpoint "pub": no private key`) {
		t.Errorf("Build(pub) error = %v, want no private key", err)
	}
	cfg, err := n.Build("priv")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PrivateKey != testKey(2) {
		t.Errorf("PrivateKey = %s, want %s", cfg.PrivateKey, testKey(2))
	}
}

// R-FILE-3, R-KEY-5: key files are relative to the network file, and read
// only when needed.
func TestPrivateKeyFile(t *testing.T) {
	path := writeConfig(t, `
endpoints:
  - name: derive
    addresses: [10.0.0.1]
    private_key_file: keys/1.key
  - name: lazy
    addresses: [10.0.0.2]
    public_key: {pub2}
    private_key_file: keys/2.key
  - name: missing
    addresses: [10.0.0.3]
    public_key: {pub3}
    private_key_file: keys/none.key
  - name: wrong
    addresses: [10.0.0.4]
    public_key: {pub4}
    private_key_file: keys/1.key
`, nil)
	keys := filepath.Join(filepath.Dir(path), "keys")
	if err := os.Mkdir(keys, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, b := range []byte{1, 2} {
		k := testKey(b)
		if err := os.WriteFile(filepath.Join(keys, fmt.Sprintf("%d.key", b)), []byte(k.String()+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// The tests run in the package directory, so this only works if the
	// key paths are relative to the network file.
	n, err := Load(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := n.Endpoint("derive").PublicKey, testKey(1).PublicKey(); got != want {
		t.Errorf("derive: PublicKey = %s, want %s", got, want)
	}

	// The file is read when the configuration is built.
	if err := os.WriteFile(filepath.Join(keys, "2.key"), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A key file that exists but is bad or wrong is an error, but a
	// missing one only means the key is not available (R-CLI-9).
	if _, err := n.Build("lazy"); err == nil || errors.Is(err, ErrNoPrivateKey) || !strings.Contains(err.Error(), "must be a base64-encoded 32-byte key") {
		t.Errorf("Build(lazy) error = %v, want a bad key", err)
	}
	if _, err := n.Build("missing"); !errors.Is(err, ErrNoPrivateKey) || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Build(missing) error = %v, want no private key and not exist", err)
	}
	if _, err := n.Build("wrong"); err == nil || errors.Is(err, ErrNoPrivateKey) || !strings.Contains(err.Error(), "does not match public_key") {
		t.Errorf("Build(wrong) error = %v, want a mismatch", err)
	}
	if _, err := n.Build("derive"); err != nil {
		t.Errorf("Build(derive): %v", err)
	}

	// Without a public key, a missing file is an error when loading.
	path = writeConfig(t, "endpoints:\n  - name: a\n    addresses: [10.0.0.1]\n    private_key_file: none.key\n", nil)
	if _, err := Load(path, Options{}); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Load error = %v, want not exist", err)
	}
}

func peerNames(peers []Peer) []string {
	var names []string
	for _, p := range peers {
		names = append(names, p.Name)
	}
	return names
}

// R-PEER-1 to R-PEER-4, R-PEER-6, R-PSK-3
func TestPeering(t *testing.T) {
	n := mustLoad(t, `
defaults:
  persistent_keepalive: 25
endpoints:
  - name: hub
    addresses: [10.0.0.1]
    private_key: {priv1}
    endpoint: hub.example.com:51820
    persistent_keepalive: 0
  - name: c
    addresses: [10.0.0.3]
    private_key: {priv3}
  - name: b
    addresses: [10.0.0.2]
    private_key: {priv2}
    peers: [hub, c]
  - name: d
    addresses: [10.0.0.4]
    private_key: {priv4}
    peers: [hub]
  - name: loner
    addresses: [10.0.0.5]
    private_key: {priv5}
    peers: []
preshared_keys:
  - endpoints: [b, hub]
    key: {priv9}
`)
	tests := map[string][]string{
		"hub":   {"c", "b", "d"},
		"c":     {"hub", "b"},
		"b":     {"hub", "c"},
		"d":     {"hub"},
		"loner": nil,
	}
	for name, want := range tests {
		cfg, err := n.Build(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := peerNames(cfg.Peers); !slices.Equal(got, want) {
			t.Errorf("%s: peers = %q, want %q", name, got, want)
		}
	}

	hub, _ := n.Build("hub")
	b, _ := n.Build("b")
	for _, p := range hub.Peers {
		if p.PersistentKeepalive != 0 {
			t.Errorf("hub's peer %s: PersistentKeepalive = %d, want 0", p.Name, p.PersistentKeepalive)
		}
		if p.Host != "" {
			t.Errorf("hub's peer %s: Host = %q, want none", p.Name, p.Host)
		}
		if (p.PresharedKey != nil) != (p.Name == "b") {
			t.Errorf("hub's peer %s: PresharedKey = %v", p.Name, p.PresharedKey)
		}
	}
	bHub := b.Peers[0]
	if bHub.PersistentKeepalive != 25 || bHub.Endpoint() != "hub.example.com:51820" {
		t.Errorf("b's peer hub = %+v", bHub)
	}
	if bHub.PresharedKey == nil || *bHub.PresharedKey != testKey(9) || *hub.Peers[1].PresharedKey != testKey(9) {
		t.Error("the preshared key is not used on both sides")
	}
}

// R-PEER-5
func TestAllowedIPConflict(t *testing.T) {
	n := mustLoad(t, `
endpoints:
  - name: a
    addresses: [10.0.0.1]
    private_key: {priv1}
  - name: b
    addresses: [10.0.0.2]
    private_key: {priv2}
    allowed_ips: [10.0.0.2, 192.168.0.0/16]
  - name: c
    addresses: [10.0.0.3]
    private_key: {priv3}
    allowed_ips: [10.0.0.3, 192.168.1.1/16]
    peers: [a]
`)
	_, err := n.Build("a")
	want := `endpoint "a": peers "b" and "c" both claim allowed IP 192.168.0.0/16`
	if err == nil || err.Error() != want {
		t.Errorf("Build(a) error = %v, want %q", err, want)
	}
	// b and c don't peer with each other, so only a is affected.
	if _, err := n.Build("b"); err != nil {
		t.Errorf("Build(b): %v", err)
	}
}

func TestBuildUnknownEndpoint(t *testing.T) {
	n := mustLoad(t, "endpoints:\n  - name: a\n    addresses: [10.0.0.1]\n    public_key: {pub1}\n")
	if _, err := n.Build("z"); err == nil || err.Error() != `unknown endpoint "z"` {
		t.Errorf("Build(z) error = %v", err)
	}
}
