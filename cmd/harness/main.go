// Command harness installs and maintains a documentation-first agent harness.
package main

import (
	"os"

	"github.com/mauricio-uy/agent-harness/internal/cli"
	"github.com/mauricio-uy/agent-harness/internal/ui"
)

func main() {
	streams := cli.IO{Out: os.Stdout, Err: os.Stderr}
	if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
		streams.SelectClients = ui.SelectClients
	}
	os.Exit(cli.Run(os.Args[1:], streams))
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
