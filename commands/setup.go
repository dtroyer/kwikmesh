// commands/setup.go - Command setup & config

package commands

import (
	"context"
	"fmt"
	"io"
	"strings"

	cli "github.com/urfave/cli/v3"

	"kwikmesh/internal/network"
	"kwikmesh/internal/render"
)

var (
	Version string
)

// RunCommands runs the km command line with args (including the program
// name in args[0]), reading input from stdin, writing normal output to
// stdout and errors to stderr. Any error is also printed to stderr, each
// line with a "km:" prefix (R-CLI-4), before it is returned.
func RunCommands(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	app := &cli.Command{
		Name:           "km",
		Version:        Version,
		Usage:          "Generate WireGuard configurations from a common YAML file",
		Reader:         stdin,
		Writer:         stdout,
		ErrWriter:      stderr,
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Load configuration from `<config-file>`",
				Value:   defaultConfig,
				Sources: cli.EnvVars("KWIKMESH_CONFIG"),
			},
		},
		Action: rootAction,
		Commands: []*cli.Command{
			generateCommand(),
			listCommand(),
			validateCommand(),
			genkeyCommand(),
			genpskCommand(),
			pubkeyCommand(),
		},
	}

	err := app.Run(context.Background(), args)
	if err != nil {
		for _, line := range strings.Split(err.Error(), "\n") {
			_, _ = fmt.Fprintf(stderr, "km: %s\n", line)
		}
	}
	return err
}

// rootAction runs when no subcommand matches: a missing command shows the
// help on stderr, and anything else is an unknown command (R-CLI-4).
func rootAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Present() {
		return fmt.Errorf("unknown command %q", cmd.Args().First())
	}
	cli.HelpPrinter(cmd.Root().ErrWriter, cli.RootCommandHelpTemplate, cmd.Root())
	return nil
}

// defaultConfig is the network file used without --config or
// KWIKMESH_CONFIG (R-FILE-1).
const defaultConfig = "config.yaml"

// loadNetwork loads the network file named by --config, checking format
// names against the renderers (R-FMT-1, R-DEV-4).
func loadNetwork(cmd *cli.Command) (*network.Network, error) {
	path := cmd.String("config")
	if path == "" {
		// KWIKMESH_CONFIG is set but empty.
		path = defaultConfig
	}
	return network.Load(path, network.Options{CheckFormat: checkFormat})
}

func checkFormat(name string) error {
	_, err := render.Lookup(name)
	return err
}

// noArgs returns an error if cmd was given any arguments.
func noArgs(cmd *cli.Command) error {
	if cmd.Args().Present() {
		return fmt.Errorf("%s: unexpected argument %q", cmd.Name, cmd.Args().First())
	}
	return nil
}
