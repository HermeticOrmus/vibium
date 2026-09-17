package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/vibium/clicker/internal/verifier"
)

func readyTestCommand(t *testing.T, scope string) *cobra.Command {
	t.Helper()
	for _, key := range []string{"VIBIUM_AI_PROVIDER", "VIBIUM_AI_MODEL", "VIBIUM_AI_BASE_URL", "VIBIUM_AI_REASONING_EFFORT", "VIBIUM_ENGINE_CHANNEL", "VIBIUM_ENGINE_VERSION", "VIBIUM_ENGINE_PATH"} {
		t.Setenv(key, "")
	}
	t.Setenv("VIBIUM_CACHE_DIR", t.TempDir())
	oldEngine, oldChannel, oldJSON := engineName, engineChannel, jsonOutput
	t.Cleanup(func() { engineName, engineChannel, jsonOutput = oldEngine, oldChannel, oldJSON })
	engineName, engineChannel, jsonOutput = "firefox", "beta", true
	exe := filepath.Join(t.TempDir(), "firefox")
	if err := os.WriteFile(exe, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIBIUM_ENGINE_PATH", exe)
	root := &cobra.Command{Use: "vibium"}
	root.PersistentFlags().StringVar(&engineName, "engine", engineName, "")
	root.PersistentFlags().StringVar(&engineChannel, "channel", engineChannel, "")
	ready := newReadyCmd()
	root.AddCommand(ready)
	if scope == "all" {
		return ready
	}
	cmd, _, err := ready.Find([]string{scope})
	if err != nil {
		t.Fatal(err)
	}
	return cmd
}

func TestReadyOptionalAIAndRequiredScopes(t *testing.T) {
	for _, scope := range []string{"all", "browser", "ai"} {
		t.Run(scope, func(t *testing.T) {
			cmd := readyTestCommand(t, scope)
			result := runReadiness(cmd, nil, func(context.Context, verifier.Config) error { t.Fatal("unconfigured AI was contacted"); return nil })
			if result.Ready != (scope != "ai") {
				t.Fatalf("unexpected readiness: %+v", result)
			}
			if (len(result.Browsers) == 0) != (scope == "ai") {
				t.Fatal("wrong browser scope")
			}
			for _, check := range result.Checks {
				if check.Name == "browser.connection" && check.Status != "skipped" {
					t.Fatal("readiness must not claim to test a browser connection")
				}
			}
			if os.Getenv("VIBIUM_ENGINE_CHANNEL") != "" {
				t.Fatal("channel defaults changed")
			}
		})
	}
}

func TestReadyReportsBrowserFailureAndStillProbesConfiguredAI(t *testing.T) {
	cmd := readyTestCommand(t, "all")
	t.Setenv("VIBIUM_AI_PROVIDER", "local")
	t.Setenv("VIBIUM_AI_MODEL", "fixture")
	t.Setenv("VIBIUM_ENGINE_PATH", filepath.Join(t.TempDir(), "missing-firefox"))
	called := false
	result := runReadiness(cmd, nil, func(context.Context, verifier.Config) error { called = true; return nil })
	if result.Ready || !called {
		t.Fatalf("unexpected readiness: %+v", result)
	}
	found := false
	for _, c := range result.Checks {
		if c.Name == "browser.installation" {
			found = c.Status == "failed" && c.Fix != ""
		}
	}
	if !found {
		t.Fatal("browser failure was not explained")
	}
}

func TestReadyProviderSelectionDoesNotMutateDefaults(t *testing.T) {
	cmd := readyTestCommand(t, "ai")
	t.Setenv("VIBIUM_AI_PROVIDER", "google")
	t.Setenv("VIBIUM_AI_MODEL", "old-model")
	if err := cmd.ParseFlags([]string{"--model", "selected-model"}); err != nil {
		t.Fatal(err)
	}
	result := runReadiness(cmd, []string{"local"}, func(_ context.Context, c verifier.Config) error {
		if c.Provider != "local" || c.Model != "selected-model" {
			t.Fatal("provider override lost")
		}
		return nil
	})
	if !result.Ready || os.Getenv("VIBIUM_AI_PROVIDER") != "google" || os.Getenv("VIBIUM_AI_MODEL") != "old-model" {
		t.Fatal("override mutated defaults")
	}
}

func hasReadyNote(notes []string, substr string) bool {
	for _, note := range notes {
		if strings.Contains(note, substr) {
			return true
		}
	}
	return false
}

func TestReadyDisplayNoteWhenEmpty(t *testing.T) {
	cmd := readyTestCommand(t, "all")
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	result := runReadiness(cmd, nil, func(context.Context, verifier.Config) error {
		t.Fatal("unconfigured AI was contacted")
		return nil
	})
	if !result.Ready {
		t.Fatalf("empty DISPLAY must not fail readiness: %+v", result)
	}
	if runtime.GOOS == "linux" && !hasReadyNote(result.Notes, "--headless") {
		t.Fatalf("missing DISPLAY note: %+v", result.Notes)
	}
	if runtime.GOOS != "linux" && hasReadyNote(result.Notes, "--headless") {
		t.Fatalf("DISPLAY note outside linux: %+v", result.Notes)
	}
}

func TestReadyDisplayNoteAbsentWhenSet(t *testing.T) {
	cmd := readyTestCommand(t, "all")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "")
	result := runReadiness(cmd, nil, func(context.Context, verifier.Config) error {
		t.Fatal("unconfigured AI was contacted")
		return nil
	})
	if hasReadyNote(result.Notes, "--headless") {
		t.Fatalf("DISPLAY note with DISPLAY set: %+v", result.Notes)
	}
}

func TestReadyDisplayNoteAbsentWhenWayland(t *testing.T) {
	cmd := readyTestCommand(t, "all")
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	result := runReadiness(cmd, nil, func(context.Context, verifier.Config) error {
		t.Fatal("unconfigured AI was contacted")
		return nil
	})
	if hasReadyNote(result.Notes, "--headless") {
		t.Fatalf("DISPLAY note with WAYLAND_DISPLAY set: %+v", result.Notes)
	}
}

func TestReadyDisplayNoteSkippedForAIScope(t *testing.T) {
	cmd := readyTestCommand(t, "ai")
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	result := runReadiness(cmd, nil, func(context.Context, verifier.Config) error {
		t.Fatal("unconfigured AI was contacted")
		return nil
	})
	if hasReadyNote(result.Notes, "--headless") {
		t.Fatal("DISPLAY note leaked into ready ai")
	}
}

func TestNodeShimNoteFromComm(t *testing.T) {
	if nodeShimNoteFromComm("node") == "" {
		t.Fatal("node parent should note the JS shim")
	}
	if nodeShimNoteFromComm("nodejs") == "" {
		t.Fatal("nodejs parent should note the JS shim")
	}
	if nodeShimNoteFromComm("bash") != "" {
		t.Fatal("bash parent should not note the JS shim")
	}
}

func TestNodeShimCmdline(t *testing.T) {
	if !nodeShimCmdline([]string{"/usr/bin/node", "/usr/lib/node_modules/vibium/bin/cli.js", "ready"}) {
		t.Fatal("cli.js argv should count as the npm shim")
	}
	if nodeShimCmdline([]string{"/usr/bin/node", "--test", "tests/cli/ready.test.js"}) {
		t.Fatal("node --test must not count as the npm shim")
	}
	script := filepath.Join(t.TempDir(), "vibium")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env node\nconsole.log(1)\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if !nodeShimCmdline([]string{"/usr/bin/node", script, "ready"}) {
		t.Fatal("shebang node script should count as the npm shim")
	}
	if nodeShimCmdline([]string{"/usr/bin/node", filepath.Join(t.TempDir(), "ready.test.js")}) {
		t.Fatal("an unrelated .js file must not count as the npm shim")
	}
}
