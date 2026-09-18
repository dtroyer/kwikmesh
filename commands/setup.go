// commands/setup.go - Command setup & config

package commands

import (
	"context"

	cli "github.com/urfave/cli/v3"
)

func RunCommands(args []string) error {
	app := &cli.Command{
		Name:           "km",
		Usage:          "Generate WireGuard configurations from a common YAML file",
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Load configuration from `<config-file>`",
				Value:   "config.yaml",
				Sources: cli.EnvVars("WGCONF_CONFIG"),
			},
		},
		Commands: []*cli.Command{},
	}

	return app.Run(context.Background(), args)
}
