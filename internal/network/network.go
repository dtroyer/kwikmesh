// internal/network/network.go - Loading and validating the network file

// Package network loads a kwikmesh network file, validates it and builds
// the configuration of each endpoint in it.
package network

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultInterface is used when neither an endpoint nor the defaults
	// set an interface (R-DEF-2).
	DefaultInterface = "wg0"
	// DefaultFormat is used when neither an endpoint nor the defaults set
	// a format (R-FMT-2).
	DefaultFormat = "wg"
)

var (
	nameRE      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	interfaceRE = regexp.MustCompile(`^[A-Za-z0-9_=+.-]{1,15}$`)
)

// Options control how a network file is loaded.
type Options struct {
	// CheckFormat, if set, returns an error for an unknown format name.
	// The formats are defined by the caller, not by this package (R-DEV-4).
	CheckFormat func(name string) error
}

// Network is a loaded and validated network file.
type Network struct {
	// Path is the file the network was loaded from.
	Path string
	// Endpoints are in the order they appear in the file.
	Endpoints []*Endpoint

	psks map[pair]Key
}

// Endpoint is one endpoint, with defaults applied.
type Endpoint struct {
	Name        string
	Description string
	Format      string
	Interface   string
	Addresses   []netip.Prefix
	// Host and Port are the address peers use to reach this endpoint.
	// Host is empty if it is not reachable (R-EP-4).
	Host string
	Port int

	ListenPort          int
	MTU                 int
	PersistentKeepalive int
	DNS                 []string
	PreUp               []string
	PostUp              []string
	PreDown             []string
	PostDown            []string

	AllowedIPs []netip.Prefix
	PublicKey  Key

	privateKey     *Key
	privateKeyFile string
	// peers is the set of endpoints this one allows, nil for all of them.
	peers map[string]bool
}

// EndpointAddr returns host:port for peers to reach e, with IPv6 hosts in
// brackets, or "" if e is not reachable.
func (e *Endpoint) EndpointAddr() string {
	return joinHostPort(e.Host, e.Port)
}

func (e *Endpoint) allows(name string) bool {
	return e.peers == nil || e.peers[name]
}

// pair is an unordered pair of endpoint names.
type pair struct{ a, b string }

func makePair(a, b string) pair {
	if a > b {
		a, b = b, a
	}
	return pair{a, b}
}

// Commands is one hook command or a list of them (R-HOOK-1).
type Commands []string

// UnmarshalYAML accepts a single string or a list of strings.
func (c *Commands) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		*c = Commands{n.Value}
		return nil
	case yaml.SequenceNode:
		s := []string{}
		if err := n.Decode(&s); err != nil {
			return err
		}
		*c = s
		return nil
	default:
		// A TypeError lets the decoder carry on and report other problems.
		return &yaml.TypeError{Errors: []string{
			fmt.Sprintf("line %d: hooks must be a command or a list of commands", n.Line),
		}}
	}
}

// settingsYAML are the fields that can be set in defaults (R-DEF-1) or on
// an endpoint. Unset fields are nil or "".
type settingsYAML struct {
	Interface           string   `yaml:"interface"`
	Format              string   `yaml:"format"`
	ListenPort          *int     `yaml:"listen_port"`
	MTU                 *int     `yaml:"mtu"`
	DNS                 []string `yaml:"dns"`
	PersistentKeepalive *int     `yaml:"persistent_keepalive"`
	PreUp               Commands `yaml:"pre_up"`
	PostUp              Commands `yaml:"post_up"`
	PreDown             Commands `yaml:"pre_down"`
	PostDown            Commands `yaml:"post_down"`
}

type endpointYAML struct {
	settingsYAML `yaml:",inline"`

	Name           string   `yaml:"name"`
	Description    string   `yaml:"description"`
	Addresses      []string `yaml:"addresses"`
	Endpoint       string   `yaml:"endpoint"`
	AllowedIPs     []string `yaml:"allowed_ips"`
	PublicKey      string   `yaml:"public_key"`
	PrivateKey     string   `yaml:"private_key"`
	PrivateKeyFile string   `yaml:"private_key_file"`
	Peers          []string `yaml:"peers"`
}

type pskYAML struct {
	Endpoints []string `yaml:"endpoints"`
	Key       string   `yaml:"key"`
}

type fileYAML struct {
	Defaults      settingsYAML   `yaml:"defaults"`
	Endpoints     []endpointYAML `yaml:"endpoints"`
	PresharedKeys []pskYAML      `yaml:"preshared_keys"`
}

// CheckInterface returns an error if name is not a valid interface name.
func CheckInterface(name string) error {
	if !interfaceRE.MatchString(name) {
		return fmt.Errorf("invalid interface name %q: use up to 15 letters, digits and _=+.-", name)
	}
	return nil
}

// Load reads, validates and resolves the network file at path. If it is
// invalid, the error lists the problems found, one per line (R-FILE-4).
func Load(path string, opts Options) (*Network, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var f fileYAML
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // R-FILE-2
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return nil, yamlError(path, err)
	}

	l := loader{path: path, dir: filepath.Dir(path), opts: opts}
	n := l.load(&f)
	if len(l.errs) > 0 {
		return nil, errors.Join(l.errs...)
	}
	return n, nil
}

var unknownFieldRE = regexp.MustCompile(`^(line \d+): field (\S+) not found in type \S+$`)

// yamlError returns err with each problem on its own line, prefixed with
// path, and unknown field messages reworded to hide Go type names.
func yamlError(path string, err error) error {
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		return fmt.Errorf("%s: %w", path, err)
	}
	errs := make([]error, len(te.Errors))
	for i, msg := range te.Errors {
		msg = unknownFieldRE.ReplaceAllString(msg, `$1: unknown field "$2"`)
		errs[i] = fmt.Errorf("%s: %s", path, msg)
	}
	return errors.Join(errs...)
}

// loader collects the problems found while loading a file.
type loader struct {
	path string
	dir  string
	opts Options
	errs []error
}

func (l *loader) addf(where, format string, args ...any) {
	l.errs = append(l.errs, fmt.Errorf("%s: %s: %s", l.path, where, fmt.Sprintf(format, args...)))
}

func (l *loader) addErr(where string, err error) {
	l.errs = append(l.errs, fmt.Errorf("%s: %s: %w", l.path, where, err))
}

func (l *loader) load(f *fileYAML) *Network {
	n := &Network{Path: l.path, psks: map[pair]Key{}}

	// R-DEF-5: defaults are checked once, here, and not again for each
	// endpoint that inherits them.
	for _, err := range l.checkSettings(&f.Defaults) {
		l.addErr("defaults", err)
	}

	if len(f.Endpoints) == 0 {
		l.addf("endpoints", "at least one endpoint is required") // R-FILE-5
	}

	known := map[string]bool{}
	for _, y := range f.Endpoints {
		known[y.Name] = true
	}

	seen := map[string]bool{}
	for i := range f.Endpoints {
		y := &f.Endpoints[i]
		where := endpointWhere(i, y.Name)
		ep, err := l.loadEndpoint(y, &f.Defaults, seen)
		if err != nil {
			l.addErr(where, err) // only the first problem (R-FILE-4)
		} else {
			n.Endpoints = append(n.Endpoints, ep)
		}
		seen[y.Name] = true

		// Every problem with a peers list is reported (R-FILE-4).
		for _, p := range y.Peers {
			switch {
			case p == y.Name:
				l.addf(where, "peers: an endpoint cannot peer with itself")
			case !known[p]:
				l.addf(where, "peers: unknown endpoint %q", p)
			}
		}
	}

	l.loadPSKs(n, f.PresharedKeys, known)
	return n
}

func endpointWhere(i int, name string) string {
	if name == "" {
		return fmt.Sprintf("endpoint #%d", i+1)
	}
	return fmt.Sprintf("endpoint %q", name)
}

// checkSettings returns every problem with the fields in s.
func (l *loader) checkSettings(s *settingsYAML) []error {
	var errs []error
	if s.Interface != "" {
		if err := CheckInterface(s.Interface); err != nil {
			errs = append(errs, fmt.Errorf("interface: %w", err))
		}
	}
	if s.Format != "" && l.opts.CheckFormat != nil {
		if err := l.opts.CheckFormat(s.Format); err != nil {
			errs = append(errs, fmt.Errorf("format: %w", err)) // R-FMT-1
		}
	}
	// R-EP-3
	if s.ListenPort != nil && (*s.ListenPort < 0 || *s.ListenPort > 65535) {
		errs = append(errs, fmt.Errorf("listen_port %d: must be between 0 and 65535", *s.ListenPort))
	}
	if s.PersistentKeepalive != nil && (*s.PersistentKeepalive < 0 || *s.PersistentKeepalive > 65535) {
		errs = append(errs, fmt.Errorf("persistent_keepalive %d: must be between 0 and 65535", *s.PersistentKeepalive))
	}
	if s.MTU != nil && *s.MTU < 0 {
		errs = append(errs, fmt.Errorf("mtu %d: must not be negative", *s.MTU))
	}
	for _, d := range s.DNS {
		if strings.TrimSpace(d) == "" {
			errs = append(errs, errors.New("dns: entries must not be empty")) // R-EP-6
			break
		}
	}
	for _, h := range []struct {
		name string
		cmds Commands
	}{{"pre_up", s.PreUp}, {"post_up", s.PostUp}, {"pre_down", s.PreDown}, {"post_down", s.PostDown}} {
		for _, c := range h.cmds {
			// R-HOOK-2
			if strings.TrimSpace(c) == "" {
				errs = append(errs, fmt.Errorf("%s: commands must not be empty", h.name))
				break
			}
			if strings.ContainsAny(c, "\r\n") {
				errs = append(errs, fmt.Errorf("%s: each command must fit on one line", h.name))
				break
			}
		}
	}
	return errs
}

// loadEndpoint checks y and applies defaults d to it. It returns the
// first problem found.
func (l *loader) loadEndpoint(y *endpointYAML, d *settingsYAML, seen map[string]bool) (*Endpoint, error) {
	// R-EP-1
	if !nameRE.MatchString(y.Name) {
		if y.Name == "" {
			return nil, errors.New("name is required")
		}
		return nil, errors.New("name must start with a letter or digit and contain only letters, digits, _, . and -")
	}
	if seen[y.Name] {
		return nil, errors.New("duplicate name")
	}
	if errs := l.checkSettings(&y.settingsYAML); len(errs) > 0 {
		return nil, errs[0]
	}

	ep := &Endpoint{
		Name:                y.Name,
		Description:         y.Description,
		Format:              first(y.Format, d.Format, DefaultFormat),
		Interface:           first(y.Interface, d.Interface, DefaultInterface),
		ListenPort:          intSetting(y.ListenPort, d.ListenPort),
		MTU:                 intSetting(y.MTU, d.MTU),
		PersistentKeepalive: intSetting(y.PersistentKeepalive, d.PersistentKeepalive), // R-DEF-3
		DNS:                 trimAll(listSetting(y.DNS, d.DNS)),
		PreUp:               trimAll(listSetting(y.PreUp, d.PreUp)),
		PostUp:              trimAll(listSetting(y.PostUp, d.PostUp)),
		PreDown:             trimAll(listSetting(y.PreDown, d.PreDown)),
		PostDown:            trimAll(listSetting(y.PostDown, d.PostDown)),
	}

	// R-EP-2
	if len(y.Addresses) == 0 {
		return nil, errors.New("at least one address is required")
	}
	for _, s := range y.Addresses {
		p, err := parsePrefix(s)
		if err != nil {
			return nil, fmt.Errorf("addresses: %w", err)
		}
		ep.Addresses = append(ep.Addresses, p)
	}

	// R-EP-5
	if y.AllowedIPs == nil {
		for _, p := range ep.Addresses {
			ep.AllowedIPs = append(ep.AllowedIPs, netip.PrefixFrom(p.Addr(), p.Addr().BitLen()))
		}
	} else {
		ep.AllowedIPs = []netip.Prefix{}
		for _, s := range y.AllowedIPs {
			p, err := parsePrefix(s)
			if err != nil {
				return nil, fmt.Errorf("allowed_ips: %w", err)
			}
			ep.AllowedIPs = append(ep.AllowedIPs, p.Masked())
		}
	}

	if y.Endpoint != "" {
		host, port, err := parseEndpoint(y.Endpoint, ep.ListenPort)
		if err != nil {
			return nil, err
		}
		ep.Host, ep.Port = host, port
	}

	if err := l.loadKeys(ep, y); err != nil {
		return nil, err
	}

	if y.Peers != nil {
		ep.peers = map[string]bool{}
		for _, p := range y.Peers {
			ep.peers[p] = true
		}
	}
	return ep, nil
}

// loadKeys sets ep's keys from y (R-KEY-2 to R-KEY-5).
func (l *loader) loadKeys(ep *Endpoint, y *endpointYAML) error {
	if y.PrivateKey != "" && y.PrivateKeyFile != "" {
		return errors.New("private_key and private_key_file are mutually exclusive")
	}

	var pub *Key
	if y.PublicKey != "" {
		k, err := ParseKey(y.PublicKey)
		if err != nil {
			return fmt.Errorf("public_key: %w", err)
		}
		pub = &k
	}

	switch {
	case y.PrivateKey != "":
		k, err := ParseKey(y.PrivateKey)
		if err != nil {
			return fmt.Errorf("private_key: %w", err)
		}
		ep.privateKey = &k
	case y.PrivateKeyFile != "":
		ep.privateKeyFile = y.PrivateKeyFile
		if !filepath.IsAbs(ep.privateKeyFile) {
			ep.privateKeyFile = filepath.Join(l.dir, ep.privateKeyFile) // R-FILE-3
		}
		if pub == nil {
			// The file is only read when its key is needed (R-KEY-5).
			k, err := readKeyFile(ep.privateKeyFile)
			if err != nil {
				return err
			}
			ep.privateKey = &k
		}
	case pub == nil:
		return errors.New("public_key, private_key or private_key_file is required")
	}

	if ep.privateKey != nil {
		ep.PublicKey = ep.privateKey.PublicKey()
		if pub != nil && *pub != ep.PublicKey {
			return errors.New("public_key does not match the private key")
		}
	} else {
		ep.PublicKey = *pub
	}
	return nil
}

func (l *loader) loadPSKs(n *Network, psks []pskYAML, known map[string]bool) {
	for i, y := range psks {
		where := fmt.Sprintf("preshared_keys entry %d", i+1)
		if len(y.Endpoints) > 0 {
			where += " (" + strings.Join(y.Endpoints, ", ") + ")"
		}

		// R-PSK-1: report every problem with the entry.
		ok := true
		if len(y.Endpoints) != 2 {
			l.addf(where, "endpoints must name exactly two endpoints")
			ok = false
		} else if y.Endpoints[0] == y.Endpoints[1] {
			l.addf(where, "endpoints must be two different endpoints")
			ok = false
		}
		for _, name := range y.Endpoints {
			if !known[name] {
				l.addf(where, "unknown endpoint %q", name)
				ok = false
			}
		}
		k, err := ParseKey(y.Key)
		if err != nil {
			l.addf(where, "key: %v", err)
			ok = false
		}
		if !ok {
			continue
		}

		p := makePair(y.Endpoints[0], y.Endpoints[1])
		if _, dup := n.psks[p]; dup {
			l.addf(where, "duplicate preshared key for this pair") // R-PSK-2
			continue
		}
		n.psks[p] = k
	}
}

// parsePrefix parses an IP prefix, keeping its host bits, or a bare IP
// address as a host prefix (R-EP-2).
func parsePrefix(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return p, fmt.Errorf("invalid prefix %q", s)
		}
		return p, nil
	}
	a, err := netip.ParseAddr(s)
	if err != nil || a.Zone() != "" {
		return netip.Prefix{}, fmt.Errorf("invalid address %q", s)
	}
	return netip.PrefixFrom(a, a.BitLen()), nil
}

// parseEndpoint parses host, host:port or [ipv6]:port, using listenPort if
// there is no port (R-EP-4).
func parseEndpoint(s string, listenPort int) (string, int, error) {
	host, portStr := s, ""
	if strings.HasPrefix(s, "[") || strings.Count(s, ":") == 1 {
		var err error
		host, portStr, err = net.SplitHostPort(s)
		if err != nil {
			return "", 0, fmt.Errorf("endpoint %q: want host, host:port or [ipv6]:port", s)
		}
	} else if strings.Contains(s, ":") {
		// More than one colon without brackets: only a bare IPv6 address.
		if a, err := netip.ParseAddr(s); err != nil || !a.Is6() || a.Zone() != "" {
			return "", 0, fmt.Errorf("endpoint %q: want host, host:port or [ipv6]:port", s)
		}
	}
	if host == "" || strings.ContainsFunc(host, func(r rune) bool { return r <= ' ' || r == '[' || r == ']' }) {
		return "", 0, fmt.Errorf("endpoint %q: invalid host", s)
	}

	if portStr == "" {
		if listenPort == 0 {
			return "", 0, fmt.Errorf("endpoint %q has no port and listen_port is not set", s)
		}
		return host, listenPort, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("endpoint %q: port must be between 1 and 65535", s)
	}
	return host, port, nil
}

func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func intSetting(ep, def *int) int {
	if ep != nil {
		return *ep
	}
	if def != nil {
		return *def
	}
	return 0
}

// listSetting returns ep if it is set, even to an empty list, and def
// otherwise; the two are never combined (R-DEF-4).
func listSetting[T ~[]string](ep, def T) []string {
	if ep != nil {
		return ep
	}
	return def
}

func trimAll(s []string) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[i] = strings.TrimSpace(v)
	}
	return out
}
