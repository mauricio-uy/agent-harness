// Package cli parses harness commands and dispatches them.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/colorprofile"

	harness "github.com/mauricio-uy/agent-harness"
	"github.com/mauricio-uy/agent-harness/internal/docs"
	"github.com/mauricio-uy/agent-harness/internal/install"
	"github.com/mauricio-uy/agent-harness/internal/ui"
)

// Version is set at build time.
var Version = "dev"

const usage = `harness installs and maintains a documentation-first agent harness.

Usage:
  harness init   [--root DIR] [--clients LIST]   install the harness and client files
  harness link   [--root DIR]                    recreate client skill links in this clone
  harness sync   [--root DIR] [--apply|--check] [SUITE...]
                                                 validate documents and regenerate indexes
  harness check  [--root DIR] [--staged] [--format text|github] [--report-dir DIR]
                                                 run every read-only check
  harness version

Suites: plans, specifications, research, runbooks (default: all).
Clients: claude-code, codex, opencode, pi; "none" installs only the base.
`

// IO carries the streams a command uses, so tests can drive it.
type IO struct {
	Out, Err io.Writer
	// SelectClients prompts for clients; nil when there is no terminal to prompt.
	SelectClients func() ([]string, error)
}

// Run executes the command in args and returns the process exit status.
func Run(args []string, streams IO) int {
	if len(args) == 0 {
		fmt.Fprint(streams.Err, usage)
		return 2
	}
	command, rest := args[0], args[1:]
	printer := ui.NewPrinter(colorprofile.NewWriter(streams.Out, os.Environ()))
	defer printer.Flush()
	streams.Out = printer
	errOut := colorprofile.NewWriter(streams.Err, os.Environ())
	var err error
	status := 0
	switch command {
	case "init":
		err = runInit(rest, streams)
		summarize(printer, err)
	case "link":
		err = runLink(rest, streams)
		summarize(printer, err)
	case "sync":
		status, err = runSync(rest, streams)
	case "check":
		status, err = runCheck(rest, streams)
	case "version", "--version":
		fmt.Fprintln(streams.Out, Version)
	case "help", "-h", "--help":
		fmt.Fprint(streams.Out, usage)
	default:
		fmt.Fprintf(streams.Err, "unknown command %q\n\n%s", command, usage)
		return 2
	}
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(errOut, ui.Failure(fmt.Sprintf("harness %s: %v", command, err)))
		return 1
	}
	return status
}

// summarize closes an init or link run with the counted actions.
func summarize(printer *ui.Printer, err error) {
	nothingDone := printer.Summary() == "no changes"
	if errors.Is(err, ui.ErrCancelled) || errors.Is(err, flag.ErrHelp) || (err != nil && nothingDone) {
		return
	}
	fmt.Fprintln(printer)
	if err != nil {
		fmt.Fprintln(printer, ui.Warning(printer.Summary()))
		return
	}
	fmt.Fprintln(printer, ui.Success(printer.Summary()))
}

func newFlags(name string, streams IO) (*flag.FlagSet, *string) {
	flags := flag.NewFlagSet("harness "+name, flag.ContinueOnError)
	flags.SetOutput(streams.Err)
	root := flags.String("root", ".", "project root")
	return flags, root
}

func absolute(root string) (string, error) {
	path, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return "", fmt.Errorf("root %s is not a directory", path)
	}
	return path, nil
}

func runInit(args []string, streams IO) error {
	flags, root := newFlags("init", streams)
	clientList := flags.String("clients", "", "comma-separated clients to configure, or none")
	if err := flags.Parse(args); err != nil {
		return err
	}
	path, err := absolute(*root)
	if err != nil {
		return err
	}
	var clients []string
	switch {
	case isFlagSet(flags, "clients"):
		clients, err = install.ParseClients(*clientList)
	case streams.SelectClients != nil:
		clients, err = streams.SelectClients()
	default:
		fmt.Fprintln(streams.Out, "No terminal to prompt; installing without clients. Pass --clients to choose them.")
	}
	if err != nil {
		return err
	}
	installer := &install.Installer{Root: path, Payload: harness.Payload, Out: streams.Out}
	return installer.Init(clients)
}

func isFlagSet(flags *flag.FlagSet, name string) bool {
	set := false
	flags.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}

func runLink(args []string, streams IO) error {
	flags, root := newFlags("link", streams)
	if err := flags.Parse(args); err != nil {
		return err
	}
	path, err := absolute(*root)
	if err != nil {
		return err
	}
	installer := &install.Installer{Root: path, Payload: harness.Payload, Out: streams.Out}
	return installer.Link()
}

func runSync(args []string, streams IO) (int, error) {
	flags, root := newFlags("sync", streams)
	apply := flags.Bool("apply", false, "write index and plan changes")
	check := flags.Bool("check", false, "write nothing; fail when synchronization is needed")
	if err := flags.Parse(args); err != nil {
		return 0, err
	}
	if *apply && *check {
		return 0, errors.New("--apply and --check are mutually exclusive")
	}
	path, err := absolute(*root)
	if err != nil {
		return 0, err
	}
	suites := flags.Args()
	if len(suites) == 0 {
		suites = docs.Suites
	}
	for _, name := range suites {
		if !slices.Contains(docs.Suites, name) {
			return 0, fmt.Errorf("unknown suite %q; choose from %s", name, strings.Join(docs.Suites, ", "))
		}
	}
	status := 0
	for _, name := range suites {
		if len(suites) > 1 {
			fmt.Fprintf(streams.Out, "\n== %s ==\n", name)
		}
		result, err := docs.SyncSuite(name, path, *apply, *check, streams.Out)
		if err != nil {
			return 0, err
		}
		status = max(status, result)
	}
	return status, nil
}

func runCheck(args []string, streams IO) (int, error) {
	flags, root := newFlags("check", streams)
	staged := flags.Bool("staged", false, "check the Git index instead of the working tree")
	format := flags.String("format", "text", "link report format: text or github")
	reportDir := flags.String("report-dir", "", "write links.json and links.md to this directory")
	if err := flags.Parse(args); err != nil {
		return 0, err
	}
	if *format != "text" && *format != "github" {
		return 0, fmt.Errorf("unknown format %q", *format)
	}
	path, err := absolute(*root)
	if err != nil {
		return 0, err
	}
	report := docs.LinkReport{GitHub: *format == "github", ReportDir: *reportDir}
	if report.ReportDir != "" {
		if report.ReportDir, err = filepath.Abs(report.ReportDir); err != nil {
			return 0, err
		}
	}
	if *staged {
		return docs.CheckStaged(path, report, streams.Out), nil
	}
	return docs.Check(path, report, streams.Out), nil
}
