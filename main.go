package main

import (
	"fmt"
	"os"

	"github.com/fabioconcina/pingolin/cmd"
	"github.com/fabioconcina/pingolin/exitcode"
)

var version = "dev"

func main() {
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if cmd.IsUnhealthy() {
			os.Exit(exitcode.Unhealthy)
		}
		os.Exit(exitcode.GeneralError)
	}
}
