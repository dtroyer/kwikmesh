// internal/render/render.go - Output formats

// Package render writes an endpoint's configuration in one of the output
// formats.
package render

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"kwikmesh/internal/network"
)

// Format is an output format (R-GEN-2).
type Format struct {
	Name string
	// Ext is the file name extension used by generate --all (R-CLI-7).
	Ext string
	// Mode is the mode files are created with (R-SEC-1).
	Mode   os.FileMode
	render func(b *bytes.Buffer, cfg *network.Config) error
}

var formats = []*Format{
	{Name: "wg", Ext: ".conf", Mode: 0o600, render: renderWG},
	{Name: "uci", Ext: ".uci", Mode: 0o600, render: renderUCI},
	{Name: "uci-script", Ext: ".sh", Mode: 0o700, render: renderUCIScript},
}

// Names returns the names of the formats.
func Names() []string {
	names := make([]string, len(formats))
	for i, f := range formats {
		names[i] = f.Name
	}
	return names
}

// Lookup returns the format called name.
func Lookup(name string) (*Format, error) {
	for _, f := range formats {
		if f.Name == name {
			return f, nil
		}
	}
	return nil, fmt.Errorf("unknown format %q (want %s)", name, strings.Join(Names(), ", "))
}

// Render returns cfg in format f.
func (f *Format) Render(cfg *network.Config) ([]byte, error) {
	var b bytes.Buffer
	if err := f.render(&b, cfg); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// hook is one wg-quick hook and its commands, in output order (R-WG-2).
type hook struct {
	key  string
	cmds []string
}

func hooks(e *network.Endpoint) []hook {
	return []hook{
		{"PreUp", e.PreUp},
		{"PostUp", e.PostUp},
		{"PreDown", e.PreDown},
		{"PostDown", e.PostDown},
	}
}

func joinPrefixes[T fmt.Stringer](ps []T, sep string) string {
	s := make([]string, len(ps))
	for i, p := range ps {
		s[i] = p.String()
	}
	return strings.Join(s, sep)
}
