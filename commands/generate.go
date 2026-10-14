// commands/generate.go - The generate command

package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cli "github.com/urfave/cli/v3"

	"kwikmesh/internal/network"
	"kwikmesh/internal/render"
)

func generateCommand() *cli.Command {
	return &cli.Command{
		Name:      "generate",
		Aliases:   []string{"gen"},
		Usage:     "Generate the configuration of one or all endpoints",
		ArgsUsage: "<endpoint>",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Usage:   "Write in `<format>` (" + strings.Join(render.Names(), ", ") + "); with --all, several may be given",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Write to `<file>` instead of stdout",
			},
			&cli.StringFlag{
				Name:    "interface",
				Aliases: []string{"i"},
				Usage:   "Use `<name>` as the interface name",
			},
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "Generate every endpoint into the config file's directory, or --dir",
			},
			&cli.StringFlag{
				Name:    "dir",
				Aliases: []string{"d"},
				Usage:   "Write files into `<dir>` when --all is used",
			},
		},
		Action: generate,
	}
}

func generate(_ context.Context, cmd *cli.Command) error {
	all := cmd.Bool("all")
	args := cmd.Args().Slice()

	// R-CLI-5, R-CLI-8
	if all {
		switch {
		case len(args) > 0:
			return errors.New("--all cannot be combined with an endpoint name")
		case cmd.IsSet("output"):
			return errors.New("--all cannot be combined with --output")
		}
	} else {
		switch {
		case cmd.IsSet("dir"):
			return errors.New("--dir is only valid with --all")
		case len(args) == 0:
			return errors.New("generate: missing endpoint name")
		case len(args) > 1:
			return fmt.Errorf("generate: takes one endpoint name, got %d", len(args))
		}
	}

	formats, err := parseFormats(cmd.StringSlice("format"))
	if err != nil {
		return err
	}
	if !all && len(formats) > 1 {
		return errors.New("only one --format is allowed without --all")
	}

	iface := cmd.String("interface")
	if iface != "" {
		if err := network.CheckInterface(iface); err != nil {
			return err
		}
	}

	n, err := loadNetwork(cmd)
	if err != nil {
		return err
	}

	if all {
		return generateAll(cmd, n, formats, iface)
	}

	cfg, err := build(n, args[0], iface)
	if err != nil {
		return err
	}
	f := formats
	if f == nil {
		f, err = endpointFormats(cfg.Endpoint)
		if err != nil {
			return err
		}
	}
	data, err := f[0].Render(cfg)
	if err != nil {
		return err
	}
	if out := cmd.String("output"); out != "" {
		return writeFileAtomic(out, data, f[0].Mode)
	}
	_, err = cmd.Root().Writer.Write(data)
	return err
}

// generateAll writes a configuration file for each endpoint to the
// directory containing the network configuration file, or into the
// directory specified by --dir.  Endpoints whose private key is not
// available are skipped, and a failure for one endpoint does not stop
// the others
// (R-CLI-6, R-CLI-9).
func generateAll(cmd *cli.Command, n *network.Network, formats []*render.Format, iface string) error {
	dir := cmd.String("dir")
	if dir == "" {
		// --dir is not present, use directory containing config file (R-CLI-6).
		dir = filepath.Dir(cmd.String("config"))
	}
	if err := os.MkdirAll(dir, 0o700); err != nil { // R-SEC-1
		return err
	}

	var errs []error
	for _, e := range n.Endpoints {
		cfg, err := build(n, e.Name, iface)
		if errors.Is(err, network.ErrNoPrivateKey) {
			_, _ = fmt.Fprintf(cmd.Root().Writer, "skipped %s: no private key\n", e.Name)
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		f := formats
		if f == nil {
			f, err = endpointFormats(e)
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}
		for _, format := range f {
			data, err := format.Render(cfg)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			path := filepath.Join(dir, e.Name+format.Ext) // R-CLI-7
			if err := writeFileAtomic(path, data, format.Mode); err != nil {
				errs = append(errs, fmt.Errorf("endpoint %q: %w", e.Name, err))
				continue
			}
			_, _ = fmt.Fprintf(cmd.Root().Writer, "wrote %s\n", path)
		}
	}
	return errors.Join(errs...)
}

func build(n *network.Network, name, iface string) (*network.Config, error) {
	cfg, err := n.Build(name)
	if err != nil {
		return nil, err
	}
	if iface != "" {
		cfg.Interface = iface // R-OUT-2
	}
	return cfg, nil
}

// endpointFormats returns the format e is generated in without --format
// (R-FMT-2).
func endpointFormats(e *network.Endpoint) ([]*render.Format, error) {
	f, err := render.Lookup(e.Format)
	if err != nil {
		return nil, fmt.Errorf("endpoint %q: %w", e.Name, err)
	}
	return []*render.Format{f}, nil
}

// parseFormats looks up the --format values, which may be repeated or
// comma-separated (R-CLI-6). It returns nil if there are none.
func parseFormats(values []string) ([]*render.Format, error) {
	var formats []*render.Format
	seen := map[string]bool{}
	for _, v := range values {
		for _, name := range strings.Split(v, ",") {
			name = strings.TrimSpace(name)
			if seen[name] {
				continue
			}
			seen[name] = true
			f, err := render.Lookup(name)
			if err != nil {
				return nil, err
			}
			formats = append(formats, f)
		}
	}
	return formats, nil
}
