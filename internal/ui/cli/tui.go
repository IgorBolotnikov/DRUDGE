package cli

import (
	"flag"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui"
)

var TuiCmd = &Cmd{
	Name: "tui",
	Desc: "Show the tasks of the project in this directory full screen",
	Setup: func(*flag.FlagSet) func(args []string) error {
		return func([]string) error { return tui.Run() }
	},
}
