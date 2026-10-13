// commands/keys.go - The genkey, genpsk and pubkey commands (R-CLI-12)

package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	cli "github.com/urfave/cli/v3"

	"kwikmesh/internal/network"
)

func genkeyCommand() *cli.Command {
	return &cli.Command{
		Name:  "genkey",
		Usage: "Generate a private key, like wg genkey",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if err := noArgs(cmd); err != nil {
				return err
			}
			return printKey(cmd, network.GeneratePrivateKey)
		},
	}
}

func genpskCommand() *cli.Command {
	return &cli.Command{
		Name:  "genpsk",
		Usage: "Generate a preshared key, like wg genpsk",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if err := noArgs(cmd); err != nil {
				return err
			}
			return printKey(cmd, network.GenerateKey)
		},
	}
}

func printKey(cmd *cli.Command, gen func() (network.Key, error)) error {
	k, err := gen()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.Root().Writer, k)
	return err
}

func pubkeyCommand() *cli.Command {
	return &cli.Command{
		Name:  "pubkey",
		Usage: "Read a private key from stdin and print its public key, like wg pubkey",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if err := noArgs(cmd); err != nil {
				return err
			}
			// A key is 44 characters; read a little more to catch junk.
			data, err := io.ReadAll(io.LimitReader(cmd.Root().Reader, 1024))
			if err != nil {
				return err
			}
			k, err := network.ParseKey(strings.TrimSpace(string(data)))
			if err != nil {
				return errors.New("pubkey: invalid private key on stdin")
			}
			_, err = fmt.Fprintln(cmd.Root().Writer, k.PublicKey())
			return err
		},
	}
}
