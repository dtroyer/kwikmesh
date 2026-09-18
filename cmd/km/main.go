// cmd/km/main.go: kwikmesh km command

package main

import (
	"fmt"
	"os"

	"kwikmesh/commands"
)

func main() {
	err := commands.RunCommands(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exit error: %+v\n", err)
		os.Exit(1)
	}
}
