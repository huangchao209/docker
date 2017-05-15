// The implementations of these functions are copied from the Go filepath
// package. The only difference is that instead of conditionally compiling
// they take in a OS variable to determine what code to run.

package pathutils

import "strings"

// volumeNameLen returns length of the leading volume name on Windows.
// It returns 0 elsewhere.
func volumeNameLen(path string, osType string) int {
	if osType != "windows" {
		return 0
	}

	if len(path) < 2 {
		return 0
	}
	// with drive letter
	c := path[0]
	if path[1] == ':' && ('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z') {
		return 2
	}
	// is it UNC? https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx
	if l := len(path); l >= 5 && isSlash(path[0]) && isSlash(path[1]) &&
		!isSlash(path[2]) && path[2] != '.' {
		// first, leading `\\` and next shouldn't be `\`. its server name.
		for n := 3; n < l-1; n++ {
			// second, next '\' shouldn't be repeated.
			if isSlash(path[n]) {
				n++
				// third, following something characters. its share name.
				if !isSlash(path[n]) {
					if path[n] == '.' {
						break
					}
					for ; n < l; n++ {
						if isSlash(path[n]) {
							break
						}
					}
					return n
				}
				break
			}
		}
	}
	return 0

}

func sameWord(a string, b string, osType string) bool {
	if osType != "windows" {
		return a == b
	}
	return strings.EqualFold(a, b)
}

func splitList(path string, osType string) []string {
	if path == "" {
		return []string{}
	}

	if osType != "windows" {
		return strings.Split(path, string(UnixListSeparator))
	}

	// The same implementation is used in LookPath in os/exec;
	// consider changing os/exec when changing this.

	// Split path, respecting but preserving quotes.
	list := []string{}
	start := 0
	quo := false
	for i := 0; i < len(path); i++ {
		switch c := path[i]; {
		case c == '"':
			quo = !quo
		case c == WindowsListSeparator && !quo:
			list = append(list, path[start:i])
			start = i + 1
		}
	}
	list = append(list, path[start:])

	// Remove quotes.
	for i, s := range list {
		if strings.Contains(s, `"`) {
			list[i] = strings.Replace(s, `"`, ``, -1)
		}
	}

	return list
}

func join(elem []string, osType string) string {
	// If there's a bug here, fix the logic in ./path_plan9.go too.
	for i, e := range elem {
		if e != "" {
			return joinNonEmpty(elem[i:], osType)
		}
	}
	return ""
}

// joinNonEmpty is like join, but it assumes that the first element is non-empty.
func joinNonEmpty(elem []string, osType string) string {
	if osType != "windows" {
		return Clean(strings.Join(elem, string(Separator(osType))), osType)
	}

	if len(elem[0]) == 2 && elem[0][1] == ':' {
		// First element is drive letter without terminating slash.
		// Keep path relative to current directory on that drive.
		return Clean(elem[0]+strings.Join(elem[1:], string(WindowsSeparator)), osType)
	}
	// The following logic prevents Join from inadvertently creating a
	// UNC path on Windows. Unless the first element is a UNC path, Join
	// shouldn't create a UNC path. See golang.org/issue/9167.
	p := Clean(strings.Join(elem, string(WindowsSeparator)), osType)
	if !isUNC(p, osType) {
		return p
	}
	// p == UNC only allowed when the first element is a UNC path.
	head := Clean(elem[0], osType)
	if isUNC(head, osType) {
		return p
	}
	// head + tail == UNC, but joining two non-UNC paths should not result
	// in a UNC path. Undo creation of UNC path.
	tail := Clean(strings.Join(elem[1:], string(WindowsSeparator)), osType)
	if head[len(head)-1] == '\\' {
		return head + tail
	}
	return head + string(WindowsSeparator) + tail
}

func isUNC(path string, osType string) bool {
	return volumeNameLen(path, osType) > 2
}

// This is not used for Linux, just Windows.
func isSlash(c uint8) bool {
	return c == '\\' || c == '/'
}
