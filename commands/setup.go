// commands/setup.go - Command setup & config

package commands

import (
	"context"
	"fmt"
	"io"

	cli "github.com/urfave/cli/v3"
)

// RunCommands runs the km command line with args (including the program
// name in args[0]), writing normal output to stdout and errors to stderr.
// Any error is also printed to stderr with a "km:" prefix (R-CLI-4)
// before it is returned.
func RunCommands(args []string, stdout, stderr io.Writer) error {
	app := &cli.Command{
		Name:           "km",
		Usage:          "Generate WireGuard configurations from a common YAML file",
		Writer:         stdout,
		ErrWriter:      stderr,
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Load configuration from `<config-file>`",
				Value:   "config.yaml",
				Sources: cli.EnvVars("KWIKMESH_CONFIG"),
			},
		},
		Action: rootAction,
	}

	err := app.Run(context.Background(), args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "km: %v\n", err)
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
