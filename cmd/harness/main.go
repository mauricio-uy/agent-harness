// Command harness installs and maintains a documentation-first agent harness.
package main

import (
	"os"

	"github.com/mauricio-uy/agent-harness/internal/cli"
)

func main() {
	info, err := os.Stdin.Stat()
	interactive := err == nil && info.Mode()&os.ModeCharDevice != 0
	os.Exit(cli.Run(os.Args[1:], cli.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Interactive: interactive}))
}
