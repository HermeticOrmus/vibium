// Package envfile loads ~/.config/vibium/ai.env into the process environment.
// Values are literals: no $VAR, $(), or backtick expansion. Explicit shell
// environment wins. The file is skipped (with a stderr warning, not a hard
// failure) when it is group- or world-readable.
package envfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/vibium/clicker/internal/paths"
)

const FileName = "ai.env"

// Status is what happened when applying ai.env. Values are never included.
type Status struct {
	Path   string
	Exists bool
	Loaded bool
	Reason string // missing, permissions, unreadable, or empty when loaded/skipped-empty
	Keys   int    // number of keys applied (not previously set)
}

var last Status

// Last reports the most recent Apply result for this process.
func Last() Status { return last }

// Path returns the ai.env path Vibium loads at start.
func Path() (string, error) {
	dir, err := paths.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// OwnerOnly reports whether mode is free of group/world bits. Windows ACLs
// are not Unix modes; treat those files as loadable.
func OwnerOnly(mode os.FileMode) bool {
	if runtime.GOOS == "windows" {
		return true
	}
	return mode.Perm()&0o077 == 0
}

// Pair is one KEY=value assignment from a dotenv file.
type Pair struct {
	Key   string
	Value string
}

// Parse reads KEY=value and export KEY=value lines. Comments and blanks are
// ignored. Surrounding quotes are stripped. The value is otherwise literal.
func Parse(content []byte) []Pair {
	var out []Pair
	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export") && (len(line) == 6 || unicode.IsSpace(rune(line[6]))) {
			line = strings.TrimSpace(line[6:])
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if !validKey(key) {
			continue
		}
		out = append(out, Pair{Key: key, Value: unquote(strings.TrimSpace(val))})
	}
	return out
}

func validKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if i == 0 && !unicode.IsLetter(r) && r != '_' {
			return false
		}
		if i > 0 && !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func unquote(v string) string {
	if len(v) >= 2 {
		if v[0] == '"' && v[len(v)-1] == '"' {
			return v[1 : len(v)-1]
		}
		if v[0] == '\'' && v[len(v)-1] == '\'' {
			return v[1 : len(v)-1]
		}
	}
	return v
}

// Apply loads ai.env into os.Environ for keys that are currently unset.
// A permissions problem is reported to warn and does not fail the caller.
func Apply(warn io.Writer) Status {
	st := Status{Reason: "missing"}
	path, err := Path()
	if err != nil {
		last = st
		return last
	}
	st.Path = path
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		last = st
		return last
	}
	if err != nil {
		st.Reason = "unreadable"
		warnf(warn, "Warning: could not read %s; Vibium did not load it.\n", path)
		last = st
		return last
	}
	st.Exists = true
	if !info.Mode().IsRegular() {
		st.Reason = "unreadable"
		warnf(warn, "Warning: %s is not a regular file; Vibium did not load it.\n", path)
		last = st
		return last
	}
	if !OwnerOnly(info.Mode()) {
		st.Reason = "permissions"
		warnf(warn, "Warning: %s is readable by others (mode %o); Vibium did not load it. Run: chmod 0600 %s\n", path, info.Mode().Perm(), path)
		last = st
		return last
	}
	body, err := os.ReadFile(path)
	if err != nil {
		st.Reason = "unreadable"
		warnf(warn, "Warning: could not read %s; Vibium did not load it.\n", path)
		last = st
		return last
	}
	st.Reason = ""
	st.Loaded = true
	for _, p := range Parse(body) {
		if _, exists := os.LookupEnv(p.Key); exists {
			continue
		}
		if err := os.Setenv(p.Key, p.Value); err != nil {
			continue
		}
		st.Keys++
	}
	last = st
	return last
}

func warnf(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, format, args...)
}
