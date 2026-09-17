package envfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseExportAndSkipComments(t *testing.T) {
	got := Parse(`
# export OPENAI_API_KEY=commented
export VIBIUM_AI_PROVIDER=openai
VIBIUM_AI_MODEL="gpt-test"
export OPENAI_API_KEY='sk-test'
PATH=/tmp
`)
	if got["VIBIUM_AI_PROVIDER"] != "openai" || got["VIBIUM_AI_MODEL"] != "gpt-test" || got["OPENAI_API_KEY"] != "sk-test" {
		t.Fatalf("%v", got)
	}
	if _, ok := got["PATH"]; ok {
		t.Fatal("loaded disallowed PATH")
	}
}

func TestParseSkipsCommandSubstitution(t *testing.T) {
	got := Parse("export OPENAI_API_KEY=$(cat /secret)\nexport ANTHROPIC_API_KEY=`cat /secret`\nexport GOOGLE_API_KEY=plain\n")
	if _, ok := got["OPENAI_API_KEY"]; ok {
		t.Fatal("expanded $(")
	}
	if _, ok := got["ANTHROPIC_API_KEY"]; ok {
		t.Fatal("expanded backticks")
	}
	if got["GOOGLE_API_KEY"] != "plain" {
		t.Fatalf("%v", got)
	}
}

func TestParseKeepsHomeLiteral(t *testing.T) {
	got := Parse("export VIBIUM_AI_BASE_URL=$HOME/v1\nexport VIBIUM_AI_MODEL=${HOME}/m\n")
	if got["VIBIUM_AI_BASE_URL"] != "$HOME/v1" || got["VIBIUM_AI_MODEL"] != "${HOME}/m" {
		t.Fatalf("%v", got)
	}
}

func TestLoadAIEnvSkipsWorldReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix modes")
	}
	dir := t.TempDir()
	t.Setenv("VIBIUM_CONFIG_DIR", dir)
	t.Setenv("VIBIUM_LOAD_AI_ENV", "")
	t.Setenv("VIBIUM_AI_MODEL", "")
	path := filepath.Join(dir, "ai.env")
	if err := os.WriteFile(path, []byte("export VIBIUM_AI_MODEL=from-file\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := LoadAIEnv(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("VIBIUM_AI_MODEL") != "" {
		t.Fatal("loaded world-readable ai.env")
	}
}

func TestLoadAIEnvFillsEmptyOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VIBIUM_CONFIG_DIR", dir)
	t.Setenv("VIBIUM_LOAD_AI_ENV", "")
	t.Setenv("VIBIUM_AI_PROVIDER", "keep-me")
	t.Setenv("VIBIUM_AI_MODEL", "")
	t.Setenv("OPENAI_API_KEY", "")
	path := filepath.Join(dir, "ai.env")
	if err := os.WriteFile(path, []byte("export VIBIUM_AI_PROVIDER=from-file\nexport VIBIUM_AI_MODEL=file-model\nexport OPENAI_API_KEY=file-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := LoadAIEnv(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("VIBIUM_AI_PROVIDER") != "keep-me" {
		t.Fatal("overwrote nonempty provider")
	}
	if os.Getenv("VIBIUM_AI_MODEL") != "file-model" || os.Getenv("OPENAI_API_KEY") != "file-key" {
		t.Fatal("did not fill empty vars")
	}
}

func TestLoadAIEnvDisabled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VIBIUM_CONFIG_DIR", dir)
	t.Setenv("VIBIUM_LOAD_AI_ENV", "0")
	t.Setenv("VIBIUM_AI_MODEL", "")
	if err := os.WriteFile(filepath.Join(dir, "ai.env"), []byte("export VIBIUM_AI_MODEL=from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := LoadAIEnv(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("VIBIUM_AI_MODEL") != "" {
		t.Fatal("loaded while disabled")
	}
}

func TestLoadAIEnvMissingFile(t *testing.T) {
	t.Setenv("VIBIUM_CONFIG_DIR", t.TempDir())
	t.Setenv("VIBIUM_LOAD_AI_ENV", "")
	if err := LoadAIEnv(); err != nil {
		t.Fatal(err)
	}
}
