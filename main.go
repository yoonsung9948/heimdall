package main

import (
	"errors"
	"os"

	"github.com/yoonsung9948/heimdall/internal/cli"
)

func main() {
	err := cli.NewRootCommand().Execute()
	if err != nil {
		var deniedErr *cli.DeniedError
		if errors.As(err, &deniedErr) {
			os.Exit(1)
		}
		var expansionErr *cli.PrivilegeExpansionError
		if errors.As(err, &expansionErr) {
			os.Exit(2)
		}
		os.Exit(2)
	}
}
