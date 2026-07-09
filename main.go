package main

import (
	"os"

	"github.com/yoonsung9948/heimdall/internal/cli"
)

func main() {
	err := cli.NewRootCommand().Execute()
	if err != nil {
		os.Exit(1)
	}
}
