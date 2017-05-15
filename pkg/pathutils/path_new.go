package pathutils

import "strings"

// Returns the os path separator given the OS
func Separator(osType string) byte {
	if osType == "windows" {
		return WindowsSeparator
	}
	return UnixSeparator
}

// Changes all instances of '/' to the os path separator
func NormalizePath(path string, osType string) string {
	return strings.Replace(path, "/", string(Separator(osType)), -1)
}
