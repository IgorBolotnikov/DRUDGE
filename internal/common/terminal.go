package common

import "os"

// IsTerminal reports whether file is a character device, such as a terminal.
func IsTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
