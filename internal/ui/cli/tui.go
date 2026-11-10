package cli

import (
	"flag"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui"
)

var TuiCmd = &Cmd{
	Name: "tui",
	Desc: "Open the full screen dashboard of the project in this directory",
	Help: "Open the full screen dashboard of the project in this directory.\n\n" +
		"The dashboard is made of tabs. Each tab arranges tiles, and each tile shows one view, like the task list.\n" +
		"The tabs and their tiles come from the TUI config file ~/.drudge/tui.json. Without it the dashboard shows the default layout.",
	Setup: func(*flag.FlagSet) func(args []string) error {
		return func([]string) error { return tui.Run() }
	},
}
