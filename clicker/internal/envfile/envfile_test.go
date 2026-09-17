package envfile

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseAcceptsExportQuotesCommentsAndLiterals(t *testing.T) {
	pairs := Parse([]byte(`
# comment
export VIBIUM_AI_PROVIDER=openai
VIBIUM_AI_MODEL="gpt-x"
OPENAI_API_KEY='$(secret)'
export EMPTY=
export SPACED = 'a b'
BACKTICKS=` + "`touch pwned`" + `
DOLLAR=$HOME
export
not-a-key=1
=novalue
`))
	got := map[string]string{}
	for _, p := range pairs {
		got[p.Key] = p.Value
	}
	want := map[string]string{
		"VIBIUM_AI_PROVIDER": "openai",
		"VIBIUM_AI_MODEL":    "gpt-x",
		"OPENAI_API_KEY":     "$(secret)",
		"EMPTY":              "",
		"SPACED":             "a b",
		"BACKTICKS":          "`touch pwned`",
		"DOLLAR":             "$HOME",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d pairs %#v, want %d", len(got), got, len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s=%q, want %q", k, got[k], v)
		}
	}
}

func TestApplySetsUnsetKeysAndLeavesShellEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VIBIUM_CONFIG_DIR", dir)
	t.Setenv("KEEP_ME", "shell")
	t.Setenv("FILE_ONLY", "temp")
	t.Setenv("OPENAI_API_KEY", "temp")
	os.Unsetenv("FILE_ONLY")
	os.Unsetenv("OPENAI_API_KEY")
	path := filepath.Join(dir, FileName)
	body := "export KEEP_ME=file\nexport FILE_ONLY=from-file\nexport OPENAI_API_KEY='$(touch pwned)'\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	st := Apply(&warn)
	if !st.Loaded || st.Reason != "" || st.Keys != 2 {
		t.Fatalf("status: %+v warn=%q", st, warn.String())
	}
	if os.Getenv("KEEP_ME") != "shell" {
		t.Fatal("shell env lost")
	}
	if os.Getenv("FILE_ONLY") != "from-file" {
		t.Fatal("unset key was not applied")
	}
	if os.Getenv("OPENAI_API_KEY") != "$(touch pwned)" {
		t.Fatalf("literal substitution was expanded: %q", os.Getenv("OPENAI_API_KEY"))
	}
	if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
		t.Fatal("command substitution ran")
	}
	if strings.Contains(warn.String(), "from-file") || strings.Contains(warn.String(), "pwned") {
		t.Fatalf("values leaked to stderr: %q", warn.String())
	}
}

func TestApplyRefusesGroupOrWorldReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix modes")
	}
	dir := t.TempDir()
	t.Setenv("VIBIUM_CONFIG_DIR", dir)
	_ = os.Unsetenv("VIBIUM_AI_PROVIDER")
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("VIBIUM_AI_PROVIDER=openai\nOPENAI_API_KEY=secret-file-marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	st := Apply(&warn)
	if st.Loaded || st.Reason != "permissions" {
		t.Fatalf("status: %+v", st)
	}
	if os.Getenv("VIBIUM_AI_PROVIDER") != "" {
		t.Fatal("world-readable file was loaded")
	}
	if !strings.Contains(warn.String(), "chmod 0600") {
		t.Fatalf("missing perms warning: %q", warn.String())
	}
	if strings.Contains(warn.String(), "secret-file-marker") {
		t.Fatal("key leaked")
	}
}

func TestApplyMissingFileIsQuiet(t *testing.T) {
	t.Setenv("VIBIUM_CONFIG_DIR", t.TempDir())
	var warn bytes.Buffer
	st := Apply(&warn)
	if st.Loaded || st.Exists || st.Reason != "missing" {
		t.Fatalf("status: %+v", st)
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected warning: %q", warn.String())
	}
}

func TestOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		if !OwnerOnly(0o666) {
			t.Fatal("windows should treat modes as loadable")
		}
		return
	}
	if !OwnerOnly(0o600) || !OwnerOnly(0o400) {
		t.Fatal("owner-only modes must load")
	}
	if OwnerOnly(0o640) || OwnerOnly(0o644) || OwnerOnly(0o666) {
		t.Fatal("group/world bits must refuse")
	}
}
