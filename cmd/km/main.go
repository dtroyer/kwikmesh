// cmd/km/main.go: kwikmesh km command

package main

import (
	"os"

	"kwikmesh/commands"
)

func main() {
	// RunCommands has already reported the error on stderr.
	if err := commands.RunCommands(os.Args, os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}
