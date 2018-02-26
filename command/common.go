package command

import "os"

// Return the locksmith data directory
func datadir() string {
	home := os.Getenv("HOME")
	return home + "/" + ".x-locksmith"
}
