// internal/network/build.go - Building an endpoint's configuration

package network

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
)

// ErrNoPrivateKey means an endpoint's private key is not available: it has
// none in the network file, or its private_key_file does not exist. This
// is allowed, since the key may be added later (R-KEY-2, R-KEY-5).
var ErrNoPrivateKey = errors.New("no private key")

// Config is everything needed to write one endpoint's configuration.
type Config struct {
	// Endpoint is the endpoint the configuration is for.
	Endpoint   *Endpoint
	PrivateKey Key
	// Interface starts as Endpoint.Interface and may be overridden
	// (R-OUT-2).
	Interface string
	Peers     []Peer
}

// Peer is one peer in an endpoint's configuration.
type Peer struct {
	Name         string
	Description  string
	PublicKey    Key
	PresharedKey *Key
	// Host and Port are where the peer is reached; Host is "" if it is
	// not reachable (R-EP-4).
	Host       string
	Port       int
	AllowedIPs []netip.Prefix
	// PersistentKeepalive comes from the endpoint being configured, not
	// from the peer (R-PEER-6).
	PersistentKeepalive int
}

// Endpoint returns host:port for reaching p, with an IPv6 host in
// brackets, or "" if p is not reachable.
func (p *Peer) Endpoint() string {
	return joinHostPort(p.Host, p.Port)
}

// Endpoint returns the endpoint called name, or nil.
func (n *Network) Endpoint(name string) *Endpoint {
	for _, e := range n.Endpoints {
		if e.Name == name {
			return e
		}
	}
	return nil
}

// Peers returns the endpoints e peers with, in file order. Two endpoints
// are peers only if each allows the other (R-PEER-1 to R-PEER-4).
func (n *Network) Peers(e *Endpoint) []*Endpoint {
	var peers []*Endpoint
	for _, p := range n.Endpoints {
		if p != e && e.allows(p.Name) && p.allows(e.Name) {
			peers = append(peers, p)
		}
	}
	return peers
}

// Build returns the configuration for the endpoint called name.
func (n *Network) Build(name string) (*Config, error) {
	e := n.Endpoint(name)
	if e == nil {
		return nil, fmt.Errorf("unknown endpoint %q", name)
	}

	priv, err := e.loadPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("endpoint %q: %w", name, err)
	}

	cfg := &Config{Endpoint: e, PrivateKey: priv, Interface: e.Interface}
	claimed := map[netip.Prefix]string{}
	for _, p := range n.Peers(e) {
		peer := Peer{
			Name:                p.Name,
			Description:         p.Description,
			PublicKey:           p.PublicKey,
			Host:                p.Host,
			Port:                p.Port,
			AllowedIPs:          p.AllowedIPs,
			PersistentKeepalive: e.PersistentKeepalive,
		}
		if k, ok := n.psks[makePair(e.Name, p.Name)]; ok {
			peer.PresharedKey = &k // R-PSK-3
		}
		// R-PEER-5
		for _, ip := range p.AllowedIPs {
			if other, dup := claimed[ip]; dup {
				return nil, fmt.Errorf("endpoint %q: peers %q and %q both claim allowed IP %s", name, other, p.Name, ip)
			}
			claimed[ip] = p.Name
		}
		cfg.Peers = append(cfg.Peers, peer)
	}
	return cfg, nil
}

// loadPrivateKey returns e's private key, reading its key file if it has
// not been read yet (R-KEY-2, R-KEY-5).
func (e *Endpoint) loadPrivateKey() (Key, error) {
	if e.privateKey != nil {
		return *e.privateKey, nil
	}
	if e.privateKeyFile == "" {
		return Key{}, fmt.Errorf("%w: set private_key or private_key_file to generate its configuration", ErrNoPrivateKey)
	}
	k, err := readKeyFile(e.privateKeyFile)
	if errors.Is(err, os.ErrNotExist) {
		return Key{}, fmt.Errorf("%w: %w", ErrNoPrivateKey, err)
	}
	if err != nil {
		return Key{}, err
	}
	if k.PublicKey() != e.PublicKey {
		return Key{}, fmt.Errorf("private_key_file %s does not match public_key", e.privateKeyFile) // R-KEY-4
	}
	e.privateKey = &k
	return k, nil
}

func joinHostPort(host string, port int) string {
	if host == "" {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}
