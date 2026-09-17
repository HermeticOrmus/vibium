package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vibium/clicker/internal/envfile"
	"github.com/vibium/clicker/internal/paths"
	"github.com/vibium/clicker/internal/verifier"
)

type setupCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

type setupResult struct {
	Ready    bool           `json:"ready"`
	Scope    string         `json:"scope,omitempty"`
	Summary  string         `json:"summary,omitempty"`
	Checks   []setupCheck   `json:"checks"`
	Notes    []string       `json:"notes"`
	Browsers []readyBrowser `json:"browsers,omitempty"`
}

// Ready owns its validation: AI-only checks must work even with invalid or
// unavailable browser settings. It never addresses a daemon session.
func isReadyCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "ready" {
			return true
		}
	}
	return false
}

func newReadyCmd() *cobra.Command {
	root := &cobra.Command{
		Annotations: map[string]string{"standalone": "true"},
		Use:         "ready", Short: "Check browser installation and AI setup",
		Long:    "Check the selected local browser executable files, then test AI when configured. No browser or driver is launched.\nDoes not install browsers or change existing sessions. Missing AI is optional here; ready ai requires it.\nAI checks make up to two model requests (API charges may apply). Loads ~/.config/vibium/ai.env at start when that file is mode 0600.",
		Example: "  vibium ready\n  # Checks browser installation and configured AI; reports fixes or READY.\n  vibium ready --json\n  # Structured readiness results; exit 0 when requested checks pass, otherwise 1.",
		Args:    cobra.NoArgs,
	}
	ai := &cobra.Command{
		Use: "ai [provider]", Short: "Test AI configuration and a provider tool round-trip without a browser",
		Long:      "Require valid AI configuration and test authentication, model access, tool calling, and a structured response.\nMakes up to two model requests (API charges may apply). Does not launch a browser.\nLoads ~/.config/vibium/ai.env at start when that file is mode 0600. Changing provider requires --model; per-call options do not change defaults.",
		Example:   "  vibium ready ai\n  # Tests the configured provider and model.\n  vibium ready ai anthropic --model your-model\n  # Tests Anthropic with the supplied model and ANTHROPIC_API_KEY.\n  vibium ready ai --json\n  # Prints the provider checks as JSON.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"openai", "anthropic", "google", "openai-compatible", "local"},
	}
	browserCmd := &cobra.Command{
		Use: "browser [engine]", Short: "Check installed browser executable files without launching them",
		Long:    "List discovered Vibium browser installations and check the selected Chrome or Firefox executable files.\nDoes not launch browsers or drivers, test BiDi connectivity, install browsers, contact AI, or touch the active daemon session.\nUse --engine and --channel for the same selection as live commands.",
		Example: "  vibium ready browser\n  # Checks the default/selected installation; other discovered installations are informational.\n  vibium ready browser firefox --channel beta\n  # Checks Firefox beta installation without launching it or changing defaults.\n  vibium ready browser chrome --json\n  # Structured installation results; browser connection is reported as skipped.",
		Args:    cobra.MaximumNArgs(1), ValidArgs: []string{"chrome", "firefox"},
	}
	root.AddCommand(ai, browserCmd)
	addModelFlags(root)
	addModelFlags(ai)
	for _, cmd := range []*cobra.Command{root, ai, browserCmd} {
		cmd.Run = func(cmd *cobra.Command, args []string) {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			cmd.SetContext(ctx)
			result := runReadiness(cmd, args, (&verifier.Model{}).Probe)
			writeReadiness(cmd, result)
			if !result.Ready {
				os.Exit(1)
			}
		}
	}
	return root
}

func runReadiness(cmd *cobra.Command, args []string, aiProbe func(context.Context, verifier.Config) error) setupResult {
	scope := cmd.Name()
	if scope == "ready" {
		scope = "all"
	}
	result := setupResult{Ready: true, Scope: scope, Checks: []setupCheck{}, Notes: readyInfoNotes(scope)}
	if scope != "ai" {
		part := checkBrowserSetup(cmd, args)
		result.Ready = part.Ready
		result.Checks = append(result.Checks, part.Checks...)
		result.Notes = append(result.Notes, part.Notes...)
		result.Browsers = part.Browsers
	}
	aiRequested := scope == "ai"
	if scope != "browser" {
		overrides := modelOverrides(cmd)
		if scope == "ai" && len(args) == 1 {
			if overrides.Provider != nil && *overrides.Provider != args[0] {
				result.Ready = false
				result.Summary = "Choose one provider and rerun vibium ready ai."
				result.Checks = append(result.Checks, setupCheck{"provider", "failed", "Provider argument conflicts with --provider.", "Choose one provider."})
				return result
			}
			overrides.Provider = &args[0]
		}
		aiRequested = aiRequested || overrides.Provider != nil || overrides.Model != nil || overrides.BaseURL != nil || overrides.ReasoningEffort != nil
		for _, key := range []string{"PROVIDER", "MODEL", "BASE_URL", "REASONING_EFFORT"} {
			aiRequested = aiRequested || os.Getenv("VIBIUM_AI_"+key) != ""
		}
		if aiRequested {
			config, _ := verifier.ResolveConfig("check", overrides)
			if config.Validate() == nil && !jsonOutput {
				fmt.Fprintln(cmd.ErrOrStderr(), "Testing AI provider (up to two model requests)...")
			}
			part := checkVerifierSetup(cmd.Context(), config, aiProbe)
			result.Ready = result.Ready && part.Ready
			result.Checks = append(result.Checks, part.Checks...)
			result.Notes = append(result.Notes, part.Notes...)
			if config.Validate() != nil {
				result.Notes = append(result.Notes, readyEnvNote()...)
			}
		} else {
			result.Checks = append(result.Checks, setupCheck{"ai", "skipped", "AI is not configured; optional for direct browser automation.", "For Run or Check, configure VIBIUM_AI_PROVIDER and VIBIUM_AI_MODEL, then run vibium ready ai."})
			result.Notes = append(result.Notes, readyEnvNote()...)
		}
	}
	retry := "vibium ready"
	if scope != "all" {
		retry += " " + scope
	}
	result.Summary = "Fix the failed checks and rerun " + retry + "."
	if result.Ready {
		switch {
		case scope == "ai":
			result.Summary = "AI configuration and provider tool round-trip passed."
		case scope == "browser" || !aiRequested:
			result.Summary = "Browser installation checks passed; launch and BiDi connectivity were not tested."
		default:
			result.Summary = "Browser installation and AI checks passed; browser launch and BiDi connectivity were not tested."
		}
	}
	return result
}

func readyInfoNotes(scope string) []string {
	notes := []string{}
	if scope != "ai" {
		if note := readyDisplayNote(); note != "" {
			notes = append(notes, note)
		}
	}
	if note := readyNodeShimNote(); note != "" {
		notes = append(notes, note)
	}
	return notes
}

func readyDisplayNote() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	if os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" {
		return ""
	}
	return "The browser is visible by default. DISPLAY and WAYLAND_DISPLAY are empty, so SSH and CI sessions need --headless or captures can be empty."
}

func readyNodeShimNote() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	// Node 24+ sets /proc/self/comm to MainThread or node-MainThread, not
	// "node". The executable path in cmdline is the stable signal.
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", os.Getppid()))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.TrimRight(string(cmdline), "\x00"), "\x00")
	if len(parts) == 0 {
		return ""
	}
	base := filepath.Base(parts[0])
	if base != "node" && base != "nodejs" {
		return ""
	}
	if !nodeShimCmdline(parts) {
		return ""
	}
	return nodeShimNoteFromComm("node")
}

func nodeShimNoteFromComm(comm string) string {
	if comm != "node" && comm != "nodejs" {
		return ""
	}
	return "This invocation is still going through the npm JS shim, so postinstall did not replace bin/cli.js with the Go binary. Run: node <package>/postinstall.js"
}

func nodeShimCmdline(parts []string) bool {
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts[1:] {
		if strings.HasPrefix(part, "-") {
			continue
		}
		base := filepath.Base(part)
		if base == "cli.js" {
			return true
		}
		if base == "vibium" && isNodeScript(part) {
			return true
		}
		break
	}
	return false
}

func isNodeScript(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 80)
	n, _ := f.Read(buf)
	line, _, _ := strings.Cut(string(buf[:n]), "\n")
	return strings.HasPrefix(line, "#!") && strings.Contains(line, "node")
}

func readyEnvNote() []string {
	dir, err := paths.GetConfigDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(dir, "ai.env")
	// Stat the real path; show the documented ~/… form. Conflating the two
	// statted a literal tilde, so the file was never found.
	shown := tildePath(path)

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return []string{"No AI settings file yet. Run: vibium setup (or vibium config init); edit " + shown + "."}
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	if !envfile.OwnerOnly(info.Mode()) {
		return []string{"Found " + shown + " but it is readable by others, so Vibium did not load it. Run: chmod 0600 " + shown + "; then rerun."}
	}
	if os.Getenv("VIBIUM_AI_PROVIDER") != "" {
		return nil
	}
	return []string{"Found " + shown + ". Set VIBIUM_AI_PROVIDER and VIBIUM_AI_MODEL in that file; Vibium loads it at start when it is mode 0600."}
}

func writeReadiness(cmd *cobra.Command, result setupResult) {
	if jsonOutput {
		envelope := jsonEnvelope{OK: result.Ready, Result: result}
		if !result.Ready {
			envelope.Error = "Requested setup checks failed"
		}
		_ = json.NewEncoder(cmd.OutOrStdout()).Encode(envelope)
		return
	}
	for _, b := range result.Browsers {
		if b.Installed {
			fmt.Fprintf(cmd.OutOrStdout(), "Installed: %s (%s) — %s\n", b.Engine, b.Channel, b.Path)
		}
	}
	for _, check := range result.Checks {
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s: %s\n", strings.ToUpper(check.Status), check.Name, check.Message)
		if check.Fix != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Fix: %s\n", check.Fix)
		}
	}
	label := "READY"
	if !result.Ready {
		label = "NOT READY"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%s: %s\n", label, result.Summary)
	for _, note := range result.Notes {
		fmt.Fprintln(cmd.OutOrStdout(), note)
	}
}
