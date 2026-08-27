package cmd

var InitCmd = &Cmd{
	Name:  "init",
	Usage: "init",
	Desc:  "Initialize a new DRUDGE project",
	Run: func(args []string) error {
		return nil
	},
}
