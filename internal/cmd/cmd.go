// Package cmd with CLI commands
package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/cmd/printer"
)

// Cmd declares one command of the drg command tree. A command with
// Subcommands is a group, any other command is a leaf.
type Cmd struct {
	Name string
	// Args names the positionals a leaf requires, in order.
	Args []string
	// Desc is the one-line summary a parent lists.
	Desc string
	// Help is the text of the command's own help. Desc stands in when it is empty.
	Help        string
	Subcommands []*Cmd
	// Setup declares the flags of the command and returns its run function.
	// A group without a run function prints its help when no subcommand is given.
	Setup func(fs *flag.FlagSet) func(args []string) error
}

const (
	versionFlagName    = "version"
	forceFlagName      = "force"
	forceFlagShortName = "f"
	// flagTerminator ends flag parsing. Every arg after it is a positional.
	flagTerminator = "--"
	// pathSeparator joins the names of a command path, as in "drg task rm".
	pathSeparator = " "
	// listFlagMarker in the usage of a flag marks a flag that takes a list.
	listFlagMarker = "comma-separated"
)

// NewRoot builds the drg command tree for a drg binary of version.
func NewRoot(version string) *Cmd {
	return &Cmd{
		Name: "drg",
		Desc: "Runs your backlog through coding agents, each in its own sandbox, on your own machine",
		Subcommands: []*Cmd{
			SetupCmd,
			CleanupCmd,
			ProjectCmd,
			TaskCmd,
			DrudgerCmd,
			NewUpdateCmd(version),
		},
		Setup: func(fs *flag.FlagSet) func(args []string) error {
			shouldPrintVersion := fs.Bool(versionFlagName, false, "Print the version of drg")
			return func(args []string) error {
				if !*shouldPrintVersion {
					return flag.ErrHelp
				}
				fmt.Printf("drg %s\n", version)
				return nil
			}
		},
	}
}

// Validate panics on a leaf without Setup and on two subcommands of one parent
// sharing a name.
func (c *Cmd) Validate() {
	c.validate(c.Name)
}

func (c *Cmd) validate(path string) {
	if len(c.Subcommands) == 0 && c.Setup == nil {
		panic(fmt.Sprintf("command %q has no Setup", path))
	}
	seen := make(map[string]bool, len(c.Subcommands))
	for _, sub := range c.Subcommands {
		if seen[sub.Name] {
			panic(fmt.Sprintf("command %q has two subcommands named %q", path, sub.Name))
		}
		seen[sub.Name] = true
		sub.validate(path + pathSeparator + sub.Name)
	}
}

// Execute walks the command tree along args and runs the command they name.
// It prints the help of that command for -h, -help and --help, and when its
// run function returns flag.ErrHelp.
func (c *Cmd) Execute(args []string) error {
	return c.execute(c.Name, args)
}

func (c *Cmd) execute(path string, args []string) error {
	fs := flag.NewFlagSet(path, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var run func(args []string) error
	if c.Setup != nil {
		run = c.Setup(fs)
	}
	repeats := refuseRepeats(fs)

	if len(c.Subcommands) > 0 {
		return c.executeGroup(path, fs, repeats, run, args)
	}
	return c.executeLeaf(path, fs, repeats, run, args)
}

func (c *Cmd) executeGroup(path string, fs *flag.FlagSet, repeats *flagRepeats, run func(args []string) error, args []string) error {
	if err := fs.Parse(args); err != nil {
		return c.handleParseError(path, fs, repeats, err)
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return c.runOrPrintHelp(path, fs, run, nil)
	}
	for _, sub := range c.Subcommands {
		if sub.Name == rest[0] {
			return sub.execute(path+pathSeparator+sub.Name, rest[1:])
		}
	}
	return fmt.Errorf("unknown subcommand %q, %s", rest[0], c.usage(path, fs))
}

func (c *Cmd) executeLeaf(path string, fs *flag.FlagSet, repeats *flagRepeats, run func(args []string) error, args []string) error {
	var positionals []string
	for {
		if err := fs.Parse(args); err != nil {
			return c.handleParseError(path, fs, repeats, err)
		}
		rest := fs.Args()
		consumed := len(args) - len(rest)
		if consumed > 0 && args[consumed-1] == flagTerminator {
			positionals = append(positionals, rest...)
			break
		}
		if len(rest) == 0 {
			break
		}
		positionals = append(positionals, rest[0])
		args = rest[1:]
	}

	if len(positionals) < len(c.Args) {
		return fmt.Errorf("%s is required, %s", c.Args[len(positionals)], c.usage(path, fs))
	}
	if len(positionals) > len(c.Args) {
		return fmt.Errorf("unexpected argument %q, %s", positionals[len(c.Args)], c.usage(path, fs))
	}
	return c.runOrPrintHelp(path, fs, run, positionals)
}

func (c *Cmd) handleParseError(path string, fs *flag.FlagSet, repeats *flagRepeats, err error) error {
	if repeats.err != nil {
		return repeats.err
	}
	if errors.Is(err, flag.ErrHelp) {
		c.printHelp(path, fs)
		return nil
	}
	return fmt.Errorf("%v, %s", err, c.usage(path, fs))
}

func (c *Cmd) runOrPrintHelp(path string, fs *flag.FlagSet, run func(args []string) error, args []string) error {
	if run == nil {
		c.printHelp(path, fs)
		return nil
	}
	if err := run(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			c.printHelp(path, fs)
			return nil
		}
		return err
	}
	return nil
}

func (c *Cmd) usage(path string, fs *flag.FlagSet) string {
	if len(c.Subcommands) > 0 {
		return "usage: " + path + " <subcommand>"
	}
	var builder strings.Builder
	builder.WriteString("usage: " + path)
	for _, arg := range c.Args {
		builder.WriteString(" <" + arg + ">")
	}
	if hasFlags(fs) {
		builder.WriteString(" [options]")
	}
	return builder.String()
}

func (c *Cmd) printHelp(path string, fs *flag.FlagSet) {
	if path == c.Name {
		printProjectName(newCommandPrinter().Theme())
	}
	fmt.Println(c.usage(path, fs))

	text := c.Help
	if text == "" {
		text = c.Desc
	}
	if text != "" {
		fmt.Println()
		fmt.Println(text)
	}

	if options := optionLines(fs); len(options) > 0 {
		fmt.Println()
		fmt.Println("Options:")
		width := 0
		for _, option := range options {
			width = max(width, len(option.label))
		}
		for _, option := range options {
			fmt.Printf("  %-*s  %s\n", width, option.label, option.usage)
		}
	}

	if len(c.Subcommands) > 0 {
		fmt.Println()
		fmt.Println("Subcommands:")
		width := 0
		for _, sub := range c.Subcommands {
			width = max(width, len(sub.Name))
		}
		for _, sub := range c.Subcommands {
			fmt.Printf("  %-*s  %s\n", width, sub.Name, sub.Desc)
		}
		fmt.Println()
		fmt.Printf("Run %s <subcommand> --help for the details of one.\n", path)
	}
}

func hasFlags(fs *flag.FlagSet) bool {
	hasAny := false
	fs.VisitAll(func(*flag.Flag) { hasAny = true })
	return hasAny
}

// optionLine is one line of the Options list: every name of one flag.Value
// and the usage text of the flag.
type optionLine struct {
	value flag.Value
	names []string
	label string
	usage string
}

// optionLines lists the flags of fs, one line per flag.Value, in the order
// fs.VisitAll visits them.
func optionLines(fs *flag.FlagSet) []*optionLine {
	var lines []*optionLine
	fs.VisitAll(func(current *flag.Flag) {
		value := unwrapOnce(current.Value)
		for _, line := range lines {
			if isSameValue(line.value, value) {
				line.names = append(line.names, current.Name)
				return
			}
		}
		lines = append(lines, &optionLine{value: value, names: []string{current.Name}})
	})

	for _, line := range lines {
		// A short alias sorts first, as in "-f, --force".
		slices.SortStableFunc(line.names, func(left, right string) int { return len(left) - len(right) })
		// The placeholder comes from the type of the value, which a wrapper hides.
		placeholder, usage := flag.UnquoteUsage(&flag.Flag{Usage: fs.Lookup(line.names[len(line.names)-1]).Usage, Value: line.value})
		labels := make([]string, len(line.names))
		for index, name := range line.names {
			labels[index] = flagLabel(name)
		}
		line.label = strings.Join(labels, ", ")
		if placeholder != "" {
			line.label += " <" + placeholder + ">"
		}
		line.usage = usage
	}
	return lines
}

// isSameValue reports whether two flags share one flag.Value. A Value of an
// incomparable type, such as the func behind flag.Func, never matches, since
// comparing it panics.
func isSameValue(left, right flag.Value) bool {
	if !reflect.TypeOf(left).Comparable() {
		return false
	}
	return left == right
}

func flagLabel(name string) string {
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

// alias registers short as another name of the flag long, on the same
// flag.Value. It panics when long is not defined yet.
func alias(fs *flag.FlagSet, short, long string) {
	longFlag := fs.Lookup(long)
	if longFlag == nil {
		panic(fmt.Sprintf("alias %q names undefined flag %q", short, long))
	}
	fs.Var(longFlag.Value, short, longFlag.Usage)
}

// flagRepeats holds the error of the first flag given more than once in the
// args of one command.
type flagRepeats struct {
	err error
}

// onceValue wraps the flag.Value of one flag name and refuses a second Set.
// The names of one value share one state, so a flag and its alias count as
// one flag.
type onceValue struct {
	flag.Value
	name    string
	state   *onceState
	repeats *flagRepeats
}

// onceState records the names one flag.Value was given under.
type onceState struct {
	value flag.Value
	names []string
	usage string
}

// refuseRepeats makes every flag of fs refuse a second value and returns the
// record of the refused flag.
func refuseRepeats(fs *flag.FlagSet) *flagRepeats {
	repeats := &flagRepeats{}
	var states []*onceState
	fs.VisitAll(func(current *flag.Flag) {
		var state *onceState
		for _, candidate := range states {
			if isSameValue(candidate.value, current.Value) {
				state = candidate
				break
			}
		}
		if state == nil {
			state = &onceState{value: current.Value, usage: current.Usage}
			states = append(states, state)
		}
		current.Value = &onceValue{Value: current.Value, name: current.Name, state: state, repeats: repeats}
	})
	return repeats
}

func (o *onceValue) Set(value string) error {
	isRepeat := len(o.state.names) > 0
	if !slices.Contains(o.state.names, o.name) {
		o.state.names = append(o.state.names, o.name)
	}
	if isRepeat {
		o.repeats.err = o.state.repeatError()
		return o.repeats.err
	}
	return o.Value.Set(value)
}

// IsBoolFlag lets a wrapped bool flag go without a value.
func (o *onceValue) IsBoolFlag() bool {
	boolValue, ok := o.Value.(interface{ IsBoolFlag() bool })
	return ok && boolValue.IsBoolFlag()
}

func (s *onceState) repeatError() error {
	labels := make([]string, len(s.names))
	for index, name := range s.names {
		labels[index] = flagLabel(name)
	}
	message := labels[0] + " is given more than once"
	if len(labels) > 1 {
		message = strings.Join(labels, " and ") + " are one flag, given more than once"
	}
	if strings.Contains(strings.ToLower(s.usage), listFlagMarker) {
		message += ", give its comma-separated values in one flag"
	}
	return errors.New(message)
}

func unwrapOnce(value flag.Value) flag.Value {
	if once, ok := value.(*onceValue); ok {
		return once.Value
	}
	return value
}

// optionalString is a string flag.Value that stays nil until the flag is set.
// It tells a flag that was not given from one given empty.
type optionalString struct {
	value *string
}

func (o *optionalString) String() string {
	return o.get()
}

func (o *optionalString) Set(value string) error {
	o.value = &value
	return nil
}

func (o *optionalString) get() string {
	if o.value == nil {
		return ""
	}
	return *o.value
}

// optionalInt is an int flag.Value that stays nil until the flag is set. It
// tells a flag that was not given from one given 0.
type optionalInt struct {
	value *int
}

func (o *optionalInt) String() string {
	if o.value == nil {
		return ""
	}
	return strconv.Itoa(*o.value)
}

func (o *optionalInt) Set(text string) error {
	value, err := strconv.Atoi(text)
	if err != nil {
		return fmt.Errorf("%q is not a whole number", text)
	}
	o.value = &value
	return nil
}

const deletionQuestion = "Are you sure? [y/N]: "

// ConfirmDeletion warns that a resource is about to be deleted, asks the user
// to confirm and reports what they answered. Anything but y or
// Y calls the deletion off.
func ConfirmDeletion(out *printer.Printer, resource string) (isConfirmed bool, err error) {
	out.Warn("This will permanently delete %s", resource)
	out.Ask(deletionQuestion)
	var response string
	if _, err := fmt.Scanln(&response); err != nil && err.Error() != "unexpected newline" {
		return false, fmt.Errorf("could not read confirmation: %w", err)
	}
	return response == "y" || response == "Y", nil
}
