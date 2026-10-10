package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/mauricio-uy/agent-harness/internal/docs"
)

const docsUsage = `Usage: harness docs COMMAND [FLAGS] [ARGS]

Find documents:
  list [FLAGS]                   filter, sort and page document metadata
  get ID [--body]                complete frontmatter and path, optionally the Markdown
  graph ID                       related documents in both directions

Change frontmatter (each sets updated to today):
  new TYPE --title TEXT          create from the template with the next free ID
  status ID STATE                change state; approved states need approval, and an
                                 approved document returns to review only by revise or revoke
  approve ID                     record the human's approval of the current revision and
                                 set status approved; only from draft or awaiting-approval
  revise ID                      start a new revision for a material change: back to draft,
                                 earlier approval kept as history, runbook validation stale;
                                 refused for superseded, cancelled, completed or retired,
                                 and for an approved ADR, which a new ADR supersedes
  revoke ID                      clear an approval the human withdrew
  touch ID                       record a body change, such as plan progress
  set ID title|owner VALUE       change the title, or a runbook owner
  relate ID TARGET...            add to related; --supersedes, --remove
  validation ID STATE            record runbook validation; --environment ENV

Types: plan, adr, use-case, functional-requirement, non-functional-requirement,
research, runbook, or their ID prefixes.

Every command prints one line of JSON, and an error as {"error":{...}} with
exit status 1. Run harness docs COMMAND --help for its flags.
`

// docsCommand describes one subcommand: its positional arguments, its own
// flags, and what it does with them.
type docsCommand struct {
	args  string // positional arguments, for help and counting
	min   int    // fewest positional arguments; -1 means exactly the named ones
	flags func(*flag.FlagSet) func(root string, args []string) (any, error)
}

var docsCommands = map[string]docsCommand{
	"list": {"", -1, func(f *flag.FlagSet) func(string, []string) (any, error) {
		bindDocFlags(f, docs.ListParameters)
		return func(root string, _ []string) (any, error) {
			var options docs.QueryOptions
			if err := decodeDocFlags(f, docs.ListParameters, &options); err != nil {
				return nil, err
			}
			return docs.List(root, options)
		}
	}},
	"get": {"ID", -1, func(f *flag.FlagSet) func(string, []string) (any, error) {
		bindDocFlags(f, docs.GetParameters)
		return func(root string, args []string) (any, error) {
			var options struct {
				Body   bool `json:"body"`
				Staged bool `json:"staged"`
			}
			if err := decodeDocFlags(f, docs.GetParameters, &options); err != nil {
				return nil, err
			}
			return docs.Get(root, args[0], options.Body, options.Staged)
		}
	}},
	"graph": {"ID", -1, func(*flag.FlagSet) func(string, []string) (any, error) {
		return func(root string, args []string) (any, error) { return docs.RelationGraph(root, args[0]) }
	}},
	"new": {"TYPE", -1, func(f *flag.FlagSet) func(string, []string) (any, error) {
		title := f.String("title", "", "single-line title (required)")
		slug := f.String("slug", "", "filename slug; derived from the title by default")
		related := f.String("related", "", "comma-separated IDs for related")
		supersedes := f.String("supersedes", "", "comma-separated IDs of the same type this replaces")
		return func(root string, args []string) (any, error) {
			n := docs.NewDocument{Type: args[0], Title: *title, Slug: *slug, Related: idList(*related), Supersedes: idList(*supersedes)}
			return editor(root).Create(n)
		}
	}},
	"status": {"ID STATE", -1, func(*flag.FlagSet) func(string, []string) (any, error) {
		return func(root string, args []string) (any, error) { return editor(root).SetStatus(args[0], args[1]) }
	}},
	"approve": {"ID", -1, func(*flag.FlagSet) func(string, []string) (any, error) {
		return func(root string, args []string) (any, error) { return editor(root).Approve(args[0]) }
	}},
	"revise": {"ID", -1, func(*flag.FlagSet) func(string, []string) (any, error) {
		return func(root string, args []string) (any, error) { return editor(root).Revise(args[0]) }
	}},
	"revoke": {"ID", -1, func(*flag.FlagSet) func(string, []string) (any, error) {
		return func(root string, args []string) (any, error) { return editor(root).Revoke(args[0]) }
	}},
	"touch": {"ID", -1, func(*flag.FlagSet) func(string, []string) (any, error) {
		return func(root string, args []string) (any, error) { return editor(root).Touch(args[0]) }
	}},
	"set": {"ID FIELD VALUE", -1, func(*flag.FlagSet) func(string, []string) (any, error) {
		return func(root string, args []string) (any, error) { return editor(root).SetField(args[0], args[1], args[2]) }
	}},
	"relate": {"ID TARGET...", 2, func(f *flag.FlagSet) func(string, []string) (any, error) {
		supersedes := f.Bool("supersedes", false, "change supersedes instead of related")
		remove := f.Bool("remove", false, "remove the targets instead of adding them")
		return func(root string, args []string) (any, error) {
			field := "related"
			if *supersedes {
				field = "supersedes"
			}
			return editor(root).Relate(args[0], field, args[1:], *remove)
		}
	}},
	"validation": {"ID STATE", -1, func(f *flag.FlagSet) func(string, []string) (any, error) {
		environment := f.String("environment", "", "where the runbook ran; required for passed, partial and failed")
		return func(root string, args []string) (any, error) {
			return editor(root).RecordValidation(args[0], args[1], *environment)
		}
	}},
}

// editor dates every change with the local calendar day, as harness date prints it.
func editor(root string) docs.Editor {
	y, m, d := now().Date()
	return docs.Editor{Root: root, Today: time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

func idList(value string) []string {
	var ids []string
	for _, id := range strings.Split(value, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func runDocs(args []string, streams IO) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(streams.Out, docsUsage)
		return 0
	}
	name := args[0]
	command, ok := docsCommands[name]
	if !ok {
		return docsFailure(streams, fmt.Errorf("unknown docs command %q; run harness docs --help", name))
	}
	flags := flag.NewFlagSet("harness docs "+name, flag.ContinueOnError)
	flags.SetOutput(streams.Err)
	root := flags.String("root", ".", "project root")
	run := command.flags(flags)
	var helpText bytes.Buffer
	flags.Usage = func() {
		helpText.Reset()
		fmt.Fprintf(&helpText, "Usage: harness docs %s [FLAGS]\n\n", strings.TrimSpace(name+" "+command.args))
		flags.SetOutput(&helpText)
		flags.PrintDefaults()
		flags.SetOutput(streams.Err)
	}
	// Flags may follow the positional arguments as well as precede them.
	if err := flags.Parse(flagsBeforePositionals(flags, args[1:])); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(streams.Out, helpText.String())
			return 0
		}
		return docsFailure(streams, err)
	}
	named := len(strings.Fields(command.args))
	if command.min < 0 && flags.NArg() != named || command.min >= 0 && flags.NArg() < command.min {
		return docsFailure(streams, fmt.Errorf("usage: harness docs %s %s", name, command.args))
	}
	path, err := absolute(queryRoot(flags, *root))
	if err != nil {
		return docsFailure(streams, err)
	}
	value, err := run(path, flags.Args())
	if err != nil {
		return docsFailure(streams, err)
	}
	if err = docs.WriteJSON(streams.Out, value); err != nil {
		fmt.Fprintln(streams.Err, err)
		return 1
	}
	return 0
}

func bindDocFlags(flags *flag.FlagSet, parameters []docs.Parameter) {
	for _, p := range parameters {
		switch p.Kind {
		case "integer":
			flags.Int(p.Name, p.Default.(int), p.Description)
		case "boolean":
			flags.Bool(p.Name, p.Default.(bool), p.Description)
		case "string":
			description := p.Description
			if len(p.Enum) > 0 {
				description += " (" + strings.Join(p.Enum, ", ") + ")"
			}
			flags.String(p.Name, p.Default.(string), description)
		}
	}
}

// Decode through the JSON field names of the options, so new parameters
// need no separate mapping from CLI flags to query options.
func decodeDocFlags(flags *flag.FlagSet, parameters []docs.Parameter, destination any) error {
	values := make(map[string]any, len(parameters))
	for _, p := range parameters {
		values[strings.ReplaceAll(p.Name, "-", "_")] = flags.Lookup(p.Name).Value.(flag.Getter).Get()
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}

func docsFailure(streams IO, err error) int {
	if _, ok := err.(*docs.QueryError); !ok {
		err = &docs.QueryError{Code: "invalid_arguments", Message: err.Error()}
	}
	if writeErr := docs.WriteJSON(streams.Out, docs.ErrorJSON(err)); writeErr != nil {
		fmt.Fprintln(streams.Err, writeErr)
	}
	return 1
}

func flagsBeforePositionals(flags *flag.FlagSet, args []string) []string {
	var options, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		options = append(options, arg)
		name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if defined := flags.Lookup(name); defined != nil && !hasValue {
			isBool := false
			if v, ok := defined.Value.(interface{ IsBoolFlag() bool }); ok {
				isBool = v.IsBoolFlag()
			}
			if !isBool && i+1 < len(args) {
				i++
				options = append(options, args[i])
			}
		}
	}
	// "--" keeps positional arguments that start with a hyphen from being read as flags.
	return append(append(options, "--"), positional...)
}
