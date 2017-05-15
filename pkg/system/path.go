package system

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/pathutils"
)

// CheckSystemDriveAndRemoveDriveLetterOS verifies and manipulates a Windows or
// Linux path. It noops if osType != Windows. Other it does the same thing
// as the CheckSystemDriveAndRemoveDriveLetter function
func CheckSystemDriveAndRemoveDriveLetterOS(path, osType string) (string, error) {
	if osType != "windows" {
		return path, nil
	}

	if len(path) == 2 && string(path[1]) == ":" {
		return "", fmt.Errorf("No relative path specified in %q", path)
	}
	if !pathutils.IsAbs(path, osType) || len(path) < 2 {
		return filepath.FromSlash(path), nil
	}
	if string(path[1]) == ":" && !strings.EqualFold(string(path[0]), "c") {
		return "", fmt.Errorf("The specified path is not on the system drive (C:)")
	}
	return filepath.FromSlash(path[2:]), nil
}
