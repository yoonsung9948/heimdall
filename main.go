package main

import "github.com/yoonsung9948/heimdall/internal/cli"

func main() {
	rootCmd := cli.NewRootCommand()
	rootCmd.Execute()
}
