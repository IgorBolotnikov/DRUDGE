package cmd

import "flag"

var CleanupCmd = &Cmd{
	Name: "cleanup",
	Desc: "Cleanup DRUDGE from this computer",
	Setup: func(fs *flag.FlagSet) func(args []string) error {
		isForced := fs.Bool(forceFlagName, false, "Remove without asking")
		alias(fs, forceFlagShortName, forceFlagName)
		return func([]string) error { return cleanup(*isForced) }
	},
}

func cleanup(isForced bool) error {
	service, err := newSetupService()
	if err != nil {
		return err
	}
	out := newCommandPrinter()

	if !isForced {
		isInstalled, err := service.IsInstalled()
		if err != nil {
			return err
		}
		if !isInstalled {
			printNothingToCleanUp(out, service.DrudgeDir())
			return nil
		}
		isConfirmed, err := ConfirmDeletion(out, service.DrudgeDir())
		if err != nil {
			return err
		}
		if !isConfirmed {
			out.skip("Left %s alone", service.DrudgeDir())
			return nil
		}
	}

	result, err := service.Cleanup()
	if err != nil {
		return err
	}
	if !result.HasRemoved {
		printNothingToCleanUp(out, result.DrudgeDir)
		return nil
	}
	out.done("Removed %s", result.DrudgeDir)
	return nil
}

func printNothingToCleanUp(out *printer, drudgeDir string) {
	out.skip("Nothing to clean up, %s does not exist", drudgeDir)
}
