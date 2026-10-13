// commands/list.go - The list and validate commands

package commands

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	cli "github.com/urfave/cli/v3"
)

func listCommand() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List the endpoints in the network file",
		Action:  list,
	}
}

// list prints a table of the endpoints (R-CLI-10).
func list(_ context.Context, cmd *cli.Command) error {
	if err := noArgs(cmd); err != nil {
		return err
	}
	n, err := loadNetwork(cmd)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tFORMAT\tADDRESSES\tENDPOINT\tPEERS")
	for _, e := range n.Endpoints {
		var addrs []string
		for _, a := range e.Addresses {
			addrs = append(addrs, a.String())
		}
		var peers []string
		for _, p := range n.Peers(e) {
			peers = append(peers, p.Name)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			e.Name, e.Format, strings.Join(addrs, ","), orDash(e.EndpointAddr()), orDash(strings.Join(peers, ",")))
	}
	return w.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func validateCommand() *cli.Command {
	return &cli.Command{
		Name:   "validate",
		Usage:  "Check the network file",
		Action: validate,
	}
}

// validate loads and checks the network file (R-CLI-11).
func validate(_ context.Context, cmd *cli.Command) error {
	if err := noArgs(cmd); err != nil {
		return err
	}
	n, err := loadNetwork(cmd)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.Root().Writer, "%s: OK\n", n.Path)
	return err
}
