package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func setupTestEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("VIBIUM_CACHE_DIR", filepath.Join(home, "cache"))
	t.Setenv("VIBIUM_CONFIG_DIR", "")
	for _, key := range []string{"VIBIUM_AI_PROVIDER", "VIBIUM_AI_MODEL", "VIBIUM_AI_BASE_URL", "VIBIUM_AI_REASONING_EFFORT", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY", "VIBIUM_ENGINE", "VIBIUM_ENGINE_CHANNEL", "VIBIUM_ENGINE_PATH", "VIBIUM_SKIP_BROWSER_DOWNLOAD"} {
		t.Setenv(key, "")
	}
	oldJSON, oldEngine, oldChannel := jsonOutput, engineName, engineChannel
	t.Cleanup(func() { jsonOutput, engineName, engineChannel = oldJSON, oldEngine, oldChannel })
	jsonOutput, engineName, engineChannel = true, "chrome", ""
	return home
}

func seedChrome(t *testing.T, home string) {
	t.Helper()
	cache := filepath.Join(home, "cache")
	version := filepath.Join(cache, "chrome-for-testing", "999.0.0.0")
	chrome := filepath.Join(version, "chrome")
	if runtime.GOOS == "darwin" {
		chrome = filepath.Join(version, "Google Chrome for Testing.app", "Contents", "MacOS", "Google Chrome for Testing")
	} else if runtime.GOOS == "windows" {
		chrome = filepath.Join(version, "chrome.exe")
	}
	if err := os.MkdirAll(filepath.Dir(chrome), 0o755); err != nil {
		t.Fatal(err)
	}
	driver := filepath.Join(version, "chromedriver")
	if runtime.GOOS == "windows" {
		driver = filepath.Join(version, "chromedriver.exe")
	}
	for _, p := range []string{chrome, driver} {
		if err := os.WriteFile(p, []byte("fixture"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

func quietUI() *setupUI {
	return &setupUI{in: &bytes.Buffer{}, out: ioDiscard(), err: ioDiscard(), interactive: false}
}

func ioDiscard() *bytes.Buffer { return &bytes.Buffer{} }

func TestSetupAINonInteractiveSkipsWhenMissingAndDoesNotOverwrite(t *testing.T) {
	home := setupTestEnv(t)
	cmd := &cobra.Command{}
	sec := setupAI(cmd, quietUI(), false)
	if sec.Status != "skipped" {
		t.Fatalf("missing file: %+v", sec)
	}
	path := filepath.Join(home, ".config", "vibium", "ai.env")
	if _, err := os.Stat(path); err == nil {
		t.Fatal("non-interactive setup wrote ai.env")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	original := "export VIBIUM_AI_PROVIDER=openai\nexport VIBIUM_AI_MODEL=keep-me\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	sec = setupAI(cmd, quietUI(), false)
	if sec.Status != "skipped" {
		t.Fatalf("existing file: %+v", sec)
	}
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Fatalf("ai.env was rewritten: %q", got)
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Fatal("unexpected backup")
	}
}

func TestSetupSkillsOnlyWhenAgentDirExists(t *testing.T) {
	home := setupTestEnv(t)
	cmd := &cobra.Command{}
	sec := setupSkills(cmd, quietUI(), false)
	if sec.Status != "skipped" {
		t.Fatalf("%+v", sec)
	}
	if _, err := os.Stat(filepath.Join(home, ".grok", "skills", "browser", "SKILL.md")); err == nil {
		t.Fatal("installed grok skills without agent dir")
	}

	if err := os.MkdirAll(filepath.Join(home, ".grok"), 0o755); err != nil {
		t.Fatal(err)
	}
	sec = setupSkills(cmd, quietUI(), false)
	if sec.Status != "done" || sec.Agent != "grok" {
		t.Fatalf("%+v", sec)
	}
	if _, err := os.Stat(filepath.Join(home, ".grok", "skills", "browser", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".grok", "skills", "check", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestSetupQuickSkipsInstalledBrowser(t *testing.T) {
	home := setupTestEnv(t)
	seedChrome(t, home)
	sec := setupBrowser(&cobra.Command{}, quietUI(), true)
	if sec.Status != "skipped" {
		t.Fatalf("%+v", sec)
	}
}

func TestSetupBrowserReportsPresent(t *testing.T) {
	home := setupTestEnv(t)
	seedChrome(t, home)
	sec := setupBrowser(&cobra.Command{}, quietUI(), false)
	if sec.Status != "done" || !strings.Contains(sec.Message, "already installed") {
		t.Fatalf("%+v", sec)
	}
}

func TestWriteAIEnvModeAndBackup(t *testing.T) {
	home := setupTestEnv(t)
	path := filepath.Join(home, ".config", "vibium", "ai.env")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("export OPENAI_API_KEY='old'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeAIEnv(path, map[string]string{"VIBIUM_AI_PROVIDER": "openai", "VIBIUM_AI_MODEL": "m", "OPENAI_API_KEY": "new"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bak), "old") {
		t.Fatal("backup lost previous contents")
	}
	body, _ := os.ReadFile(path)
	if strings.Contains(string(body), "old") || !strings.Contains(string(body), "export VIBIUM_AI_PROVIDER='openai'") {
		t.Fatalf("written file: %s", body)
	}
}

func TestParseProviderChoice(t *testing.T) {
	got, err := parseProviderChoice("2", "openai")
	if err != nil || got != "anthropic" {
		t.Fatalf("got %q %v", got, err)
	}
	got, err = parseProviderChoice("local", "openai")
	if err != nil || got != "local" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := parseProviderChoice("nope", "openai"); err == nil {
		t.Fatal("accepted unknown provider")
	}
}
