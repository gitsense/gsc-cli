/*
 * Component: Main Entry Point
 * Block-UUID: a68f96fd-f991-42da-b84f-1215ec016a02
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: The main entry point for the gsc CLI application, initializing and executing the root command.
 * Language: Go
 * Created-at: 2026-02-02T05:30:03.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package main

import (
	"github.com/yourusername/gsc-cli/internal/cli"
)

func main() {
	cli.HandleExit(cli.Execute())
}
