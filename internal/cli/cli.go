// Package cli parses harness commands and dispatches them.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/colorprofile"

	harness "github.com/mauricio-uy/agent-harness"
	"github.com/mauricio-uy/agent-harness/internal/docs"
	"github.com/mauricio-uy/agent-harness/internal/install"
	"github.com/mauricio-uy/agent-harness/internal/report"
	"github.com/mauricio-uy/agent-harness/internal/ui"
)

// Version is set at build time.
var Version = "dev"

// now is replaceable so tests can fix the date.
var now = time.Now

const usage = `harness installs and maintains a documentation-first agent harness.

Usage:
  harness init   [--root DIR] [--clients LIST]   install the harness and client files
  harness link   [--root DIR]                    recreate client skill links in this clone
  harness sync   [--root DIR] [--apply|--check] [SUITE...]
                                                 validate documents and regenerate indexes
  harness check  [--root DIR] [--staged] [--format text|github|json] [--report-dir DIR]
                                                 run every read-only check
  harness id     [--root DIR] TYPE                print the next free ID of a document type
  harness date                                    print today's date as YYYY-MM-DD
  harness version

Suites: plans, specifications, research, runbooks (default: all).
Types: plan, adr, use-case, functional-requirement, non-functional-requirement,
research, runbook, or their ID prefixes.
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
	errOut := colorprofile.NewWriter(streams.Err, os.Environ())
	var err error
	status := 0
	switch command {
	case "init":
		err = runInit(rest, streams, printer)
		summarize(printer, err)
	case "link":
		err = runLink(rest, streams, printer)
		summarize(printer, err)
	case "sync":
		status, err = runSync(rest, streams, printer)
	case "check":
		status, err = runCheck(rest, streams, printer)
	case "id":
		err = runID(rest, streams)
	case "date":
		err = runDate(rest, streams)
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
	report.Blank(printer)
	if err != nil {
		printer.Emit(report.Plain, ui.Warning(printer.Summary()))
		return
	}
	printer.Emit(report.Plain, ui.Success(printer.Summary()))
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

func runInit(args []string, streams IO, out report.Sink) error {
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
		out.Emit(report.Plain, "No terminal to prompt; installing without clients. Pass --clients to choose them.")
	}
	if err != nil {
		return err
	}
	installer := &install.Installer{Root: path, Payload: harness.Payload, Out: out}
	return installer.Init(clients)
}

func isFlagSet(flags *flag.FlagSet, name string) bool {
	set := false
	flags.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}

func runLink(args []string, streams IO, out report.Sink) error {
	flags, root := newFlags("link", streams)
	if err := flags.Parse(args); err != nil {
		return err
	}
	path, err := absolute(*root)
	if err != nil {
		return err
	}
	installer := &install.Installer{Root: path, Payload: harness.Payload, Out: out}
	return installer.Link()
}

func runSync(args []string, streams IO, out report.Sink) (int, error) {
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
			report.Blank(out)
			out.Emit(report.Section, name)
		}
		result, err := docs.SyncSuite(name, path, *apply, *check, out)
		if err != nil {
			return 0, err
		}
		status = max(status, result)
	}
	return status, nil
}

func runCheck(args []string, streams IO, out report.Sink) (int, error) {
	flags, root := newFlags("check", streams)
	staged := flags.Bool("staged", false, "check the Git index instead of the working tree")
	format := flags.String("format", "text", "output format: text, github, or json")
	reportDir := flags.String("report-dir", "", "write links.json and links.md to this directory")
	if err := flags.Parse(args); err != nil {
		return 0, err
	}
	if !slices.Contains([]string{"text", "github", "json"}, *format) {
		return 0, fmt.Errorf("unknown format %q", *format)
	}
	path, err := absolute(*root)
	if err != nil {
		return 0, err
	}
	links := docs.LinkReport{GitHub: *format == "github", ReportDir: *reportDir}
	if links.ReportDir != "" {
		if links.ReportDir, err = filepath.Abs(links.ReportDir); err != nil {
			return 0, err
		}
	}
	var result docs.CheckResult
	if *staged {
		if result, err = docs.InspectStaged(path); err != nil {
			return 0, err
		}
	} else {
		result = docs.Inspect(path)
	}
	if *format != "json" {
		return result.Report(links, out), nil
	}
	if links.ReportDir != "" {
		if err := docs.WriteLinkReports(links.ReportDir, result.Links.Scanned, result.Links.Errors); err != nil {
			return 0, err
		}
	}
	encoder := json.NewEncoder(streams.Out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return 0, err
	}
	if !result.OK {
		return 1, nil
	}
	return 0, nil
}

// runID prints the next free ID of a document type, for an agent to use as is.
func runID(args []string, streams IO) error {
	flags, root := newFlags("id", streams)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("name one document type: %s", strings.Join(docs.TypeNames(), ", "))
	}
	path, err := absolute(*root)
	if err != nil {
		return err
	}
	id, err := docs.NextID(path, flags.Arg(0))
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Out, id)
	return nil
}

// runDate prints the system's local date, so an agent never guesses it.
func runDate(args []string, streams IO) error {
	flags := flag.NewFlagSet("harness date", flag.ContinueOnError)
	flags.SetOutput(streams.Err)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("date takes no arguments")
	}
	fmt.Fprintln(streams.Out, now().Format(time.DateOnly))
	return nil
}
