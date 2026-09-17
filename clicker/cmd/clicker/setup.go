package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vibium/clicker/internal/browser"
	"github.com/vibium/clicker/internal/envfile"
	"github.com/vibium/clicker/internal/verifier"
)

var setupProviders = []string{"openai", "anthropic", "google", "openai-compatible", "local"}

type setupSection struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Path    string   `json:"path,omitempty"`
	Paths   []string `json:"paths,omitempty"`
	Agent   string   `json:"agent,omitempty"`
}

type setupCommandResult struct {
	Sections []setupSection `json:"sections"`
	Ready    *setupResult   `json:"ready,omitempty"`
}

func isSetupCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "setup" {
			return true
		}
	}
	return false
}

func newSetupCmd() *cobra.Command {
	var nonInteractive, quick bool
	cmd := &cobra.Command{
		Annotations: map[string]string{"standalone": "true"},
		Use:         "setup [browser|ai|skills]",
		Short:       "Configure browser, AI, and agent skills",
		Long: `Interactive setup for the local browser, AI settings file, and agent skills.

Sections can be run on their own: vibium setup browser|ai|skills.
--non-interactive skips prompts (also used when stdin is not a TTY).
--quick fills only what is missing.
After the selected sections, setup runs the matching vibium ready checks.
Vibium loads ~/.config/vibium/ai.env at start when the file is mode 0600.`,
		Example: `  vibium setup
  # Browser, AI, skills, then readiness.

  vibium setup --non-interactive
  # No prompts. Installs a missing browser. Leaves an existing ai.env alone.

  vibium setup --quick
  # Only fill what is missing.

  vibium setup ai
  # Provider, model, and API key only.`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"browser", "ai", "skills"},
		Run: func(cmd *cobra.Command, args []string) {
			section := ""
			if len(args) == 1 {
				section = args[0]
			}
			runSetup(cmd, section, nonInteractive, quick)
		},
	}
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Do not prompt; skip sections that need input")
	cmd.Flags().BoolVar(&quick, "quick", false, "Only fill what is missing")
	return cmd
}

func runSetup(cmd *cobra.Command, section string, nonInteractive, quick bool) {
	if jsonOutput {
		browser.Progress = os.Stderr
	}
	interactive := !nonInteractive && !jsonOutput && inputIsTTY(cmd)
	ui := newSetupUI(cmd, interactive)
	ui.banner()

	want := map[string]bool{"browser": true, "ai": true, "skills": true}
	if section != "" {
		if section != "browser" && section != "ai" && section != "skills" {
			err := fmt.Errorf("unknown setup section %q; choose browser, ai, or skills", section)
			if jsonOutput {
				printJSON(jsonEnvelope{OK: false, Error: err.Error()})
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
			}
			os.Exit(1)
		}
		want = map[string]bool{section: true}
	}

	result := setupCommandResult{}
	if want["browser"] {
		ui.heading("Browser")
		result.Sections = append(result.Sections, setupBrowser(cmd, ui, quick))
	}
	if want["ai"] {
		ui.heading("AI")
		result.Sections = append(result.Sections, setupAI(cmd, ui, quick))
	}
	if want["skills"] {
		ui.heading("Skills")
		result.Sections = append(result.Sections, setupSkills(cmd, ui, quick))
	}

	readyScope := "all"
	switch section {
	case "browser":
		readyScope = "browser"
	case "ai":
		readyScope = "ai"
	case "skills":
		readyScope = ""
	}
	if readyScope != "" {
		ready := runSetupReadiness(cmd, readyScope)
		result.Ready = &ready
		if !jsonOutput {
			writeReadiness(cmd, ready)
		}
	}

	ok := true
	var failed []string
	for _, s := range result.Sections {
		if s.Status == "failed" {
			ok = false
			failed = append(failed, s.Name)
		}
	}
	if jsonOutput {
		env := jsonEnvelope{OK: ok, Result: result}
		if !ok {
			env.Error = "Setup failed: " + strings.Join(failed, ", ")
		}
		printJSON(env)
	} else if !ok {
		fmt.Fprintf(cmd.ErrOrStderr(), "Setup failed: %s\n", strings.Join(failed, ", "))
	}
	if !ok {
		os.Exit(1)
	}
}

func setupBrowser(cmd *cobra.Command, ui *setupUI, quick bool) setupSection {
	engine := engineName
	if engine == "" {
		engine = defaultEngine()
	}
	if engine != "chrome" && engine != "firefox" {
		return setupSection{Name: "browser", Status: "failed", Message: fmt.Sprintf("unsupported engine %q (supported: chrome, firefox)", engine)}
	}
	if quick && browser.EngineInstalled(engine) {
		ui.skip("%s is already installed; skipping (--quick).", engine)
		return setupSection{Name: "browser", Status: "skipped", Message: engine + " is already installed."}
	}
	if browser.EngineInstalled(engine) {
		ui.ok("%s is already installed.", engine)
		part := checkBrowserSetup(cmd, nil)
		if !part.Ready {
			return setupSection{Name: "browser", Status: "failed", Message: browserSectionMessage(part)}
		}
		path := selectedBrowserPath(part)
		return setupSection{Name: "browser", Status: "done", Message: engine + " is already installed.", Path: path}
	}
	ui.println("Installing %s...", engine)
	var installPath string
	var err error
	if engine == "firefox" {
		installPath, err = browser.InstallFirefox()
	} else {
		result, ierr := browser.Install()
		err = ierr
		if result != nil {
			installPath = result.ChromePath
		}
	}
	if err != nil {
		return setupSection{Name: "browser", Status: "failed", Message: err.Error()}
	}
	part := checkBrowserSetup(cmd, nil)
	if !part.Ready {
		return setupSection{Name: "browser", Status: "failed", Message: browserSectionMessage(part), Path: installPath}
	}
	if installPath == "" {
		installPath = selectedBrowserPath(part)
	}
	ui.ok("Installed %s.", engine)
	return setupSection{Name: "browser", Status: "done", Message: "Installed " + engine + ".", Path: installPath}
}

func selectedBrowserPath(part setupResult) string {
	for _, b := range part.Browsers {
		if b.Selected {
			return b.Path
		}
	}
	return ""
}

func browserSectionMessage(part setupResult) string {
	for _, c := range part.Checks {
		if c.Status == "failed" {
			if c.Fix != "" {
				return c.Message + " " + c.Fix
			}
			return c.Message
		}
	}
	return "browser setup failed"
}

func setupAI(cmd *cobra.Command, ui *setupUI, quick bool) setupSection {
	path, err := envfile.Path()
	if err != nil {
		return setupSection{Name: "ai", Status: "failed", Message: err.Error()}
	}
	shown := tildePath(path)
	if quick && aiConfigValid() {
		ui.skip("AI settings are already valid; skipping (--quick).")
		return setupSection{Name: "ai", Status: "skipped", Message: "AI settings are already valid.", Path: path}
	}
	if !ui.interactive {
		if _, err := os.Stat(path); err == nil {
			ui.skip("Left existing %s unchanged (non-interactive).", shown)
			return setupSection{Name: "ai", Status: "skipped", Message: "Existing ai.env was left unchanged.", Path: path}
		}
		ui.skip("Skipped AI: no prompts in non-interactive mode. Run vibium setup ai in a terminal, or vibium config init.")
		return setupSection{Name: "ai", Status: "skipped", Message: "No ai.env yet; non-interactive setup does not write credentials."}
	}

	provider := os.Getenv("VIBIUM_AI_PROVIDER")
	if provider == "" {
		provider = "openai"
	}
	ui.println("%s", maybePaint(ui.useColor(), brandText, "AI provider:"))
	defIdx := 1
	for i, p := range setupProviders {
		num := maybePaint(ui.useColor(), brandAccent, fmt.Sprintf("%d)", i+1))
		ui.println("  %s %s", num, p)
		if p == provider {
			defIdx = i + 1
		}
	}
	choice, err := ui.prompt("Provider", fmt.Sprintf("%d", defIdx))
	if err != nil {
		return setupSection{Name: "ai", Status: "failed", Message: err.Error()}
	}
	provider, err = parseProviderChoice(choice, provider)
	if err != nil {
		return setupSection{Name: "ai", Status: "failed", Message: err.Error()}
	}

	modelDefault := os.Getenv("VIBIUM_AI_MODEL")
	if modelDefault == "" && provider == "openai" {
		modelDefault = "gpt-5.6-sol"
	}
	model, err := ui.prompt("Model", modelDefault)
	if err != nil {
		return setupSection{Name: "ai", Status: "failed", Message: err.Error()}
	}
	if strings.TrimSpace(model) == "" {
		return setupSection{Name: "ai", Status: "failed", Message: "A model is required."}
	}

	credVar := verifier.Config{Provider: provider}.CredentialVariable()
	existingKey := os.Getenv(credVar)
	needsKey := provider == "openai" || provider == "anthropic" || provider == "google"
	key := existingKey
	if needsKey || existingKey != "" {
		label := credVar
		if existingKey != "" {
			label += " [saved]"
		}
		entered, err := ui.promptSecret(label)
		if err != nil {
			return setupSection{Name: "ai", Status: "failed", Message: err.Error()}
		}
		if entered != "" {
			key = entered
		}
	}

	baseURL := os.Getenv("VIBIUM_AI_BASE_URL")
	if provider == "openai-compatible" || provider == "local" {
		if baseURL == "" && provider == "local" {
			baseURL = "http://127.0.0.1:8080/v1"
		}
		baseURL, err = ui.prompt("API base URL", baseURL)
		if err != nil {
			return setupSection{Name: "ai", Status: "failed", Message: err.Error()}
		}
	} else {
		baseURL = ""
	}

	effort := ""
	if provider == "openai" || provider == "openai-compatible" {
		effort = os.Getenv("VIBIUM_AI_REASONING_EFFORT")
		if effort == "" && provider == "openai" {
			effort = "none"
		}
	}

	kv := map[string]string{
		"VIBIUM_AI_PROVIDER": provider,
		"VIBIUM_AI_MODEL":    model,
	}
	if key != "" {
		kv[credVar] = key
	}
	if baseURL != "" {
		kv["VIBIUM_AI_BASE_URL"] = baseURL
	}
	if effort != "" {
		kv["VIBIUM_AI_REASONING_EFFORT"] = effort
	}
	if err := writeAIEnv(path, kv); err != nil {
		return setupSection{Name: "ai", Status: "failed", Message: err.Error()}
	}
	applyWrittenAI(kv)
	ui.ok("Wrote %s (0600).", shown)
	return setupSection{Name: "ai", Status: "done", Message: "Wrote AI settings.", Path: path}
}

func setupSkills(cmd *cobra.Command, ui *setupUI, quick bool) setupSection {
	agent, present := setupSkillAgent()
	if !ui.interactive && !present {
		ui.skip("Skipped skills: no ~/.grok or ~/.claude directory.")
		return setupSection{Name: "skills", Status: "skipped", Message: "No ~/.grok or ~/.claude directory; skills were not installed."}
	}
	if !present {
		agent = defaultSkillAgent()
	}
	if quick && skillsPresent(agent) {
		ui.skip("Skills already installed for %s; skipping (--quick).", agent)
		return setupSection{Name: "skills", Status: "skipped", Message: "Skills already installed.", Agent: agent}
	}
	if ui.interactive {
		ok, err := ui.confirm(fmt.Sprintf("Install browser and check skills for %s?", agent), true)
		if err != nil {
			return setupSection{Name: "skills", Status: "failed", Message: err.Error(), Agent: agent}
		}
		if !ok {
			ui.skip("Skipped skills.")
			return setupSection{Name: "skills", Status: "skipped", Message: "Skills install declined.", Agent: agent}
		}
	}

	var paths []string
	for _, name := range []string{"browser", "check"} {
		dir, skillPath, err := writeSkill(name, agent)
		if err != nil {
			return setupSection{Name: "skills", Status: "failed", Message: err.Error(), Agent: agent, Paths: paths}
		}
		paths = append(paths, skillPath)
		ui.ok("Installed %s skill to %s.", name, dir)
	}
	return setupSection{Name: "skills", Status: "done", Message: "Installed browser and check skills.", Agent: agent, Paths: paths}
}

func runSetupReadiness(cmd *cobra.Command, scope string) setupResult {
	ready := newReadyCmd()
	target := ready
	if scope == "ai" || scope == "browser" {
		found, _, err := ready.Find([]string{scope})
		if err == nil {
			target = found
		}
	}
	target.SetContext(cmd.Context())
	probe := (&verifier.Model{}).Probe
	config, _ := verifier.ResolveConfig("check", verifier.Overrides{})
	if strings.TrimSpace(config.APIKey) == "" {
		probe = func(context.Context, verifier.Config) error { return nil }
	}
	result := runReadiness(target, nil, probe)
	if strings.TrimSpace(config.APIKey) == "" {
		for i, c := range result.Checks {
			if c.Name == "provider" && c.Status == "passed" {
				result.Checks[i].Status = "skipped"
				result.Checks[i].Message = "No API key; provider was not contacted."
			}
		}
	}
	return result
}

func aiConfigValid() bool {
	config, err := verifier.ConfigFromEnv()
	return err == nil && config.Validate() == nil
}

func parseProviderChoice(choice, fallback string) (string, error) {
	choice = strings.TrimSpace(strings.ToLower(choice))
	if choice == "" {
		return fallback, nil
	}
	if n := 0; len(choice) == 1 && choice[0] >= '1' && choice[0] <= '5' {
		n = int(choice[0] - '0')
		return setupProviders[n-1], nil
	}
	for _, p := range setupProviders {
		if choice == p {
			return p, nil
		}
	}
	return "", fmt.Errorf("unknown provider %q; choose openai, anthropic, google, openai-compatible, or local", choice)
}

func writeAIEnv(path string, kv map[string]string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("could not create %s: %w", dir, err)
	}
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, path+".bak"); err != nil {
			return fmt.Errorf("could not back up %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("could not check %s: %w", path, err)
	}
	var b strings.Builder
	b.WriteString("# Written by vibium setup. Mode 0600. Vibium loads this file at start.\n")
	order := []string{"VIBIUM_AI_PROVIDER", "VIBIUM_AI_MODEL", "VIBIUM_AI_BASE_URL", "VIBIUM_AI_REASONING_EFFORT", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY"}
	written := map[string]bool{}
	for _, k := range order {
		if v, ok := kv[k]; ok {
			b.WriteString("export " + k + "=" + shellSingleQuote(v) + "\n")
			written[k] = true
		}
	}
	for k, v := range kv {
		if written[k] {
			continue
		}
		b.WriteString("export " + k + "=" + shellSingleQuote(v) + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("could not set permissions on %s: %w", path, err)
	}
	return nil
}

func applyWrittenAI(kv map[string]string) {
	for _, k := range []string{"VIBIUM_AI_PROVIDER", "VIBIUM_AI_MODEL", "VIBIUM_AI_BASE_URL", "VIBIUM_AI_REASONING_EFFORT"} {
		if _, ok := kv[k]; !ok {
			_ = os.Unsetenv(k)
		}
	}
	for k, v := range kv {
		_ = os.Setenv(k, v)
	}
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func defaultSkillAgent() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "claude"
	}
	if isDir(filepath.Join(home, ".grok")) {
		return "grok"
	}
	return "claude"
}

func setupSkillAgent() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	if isDir(filepath.Join(home, ".grok")) {
		return "grok", true
	}
	if isDir(filepath.Join(home, ".claude")) {
		return "claude", true
	}
	return "", false
}

func skillsPresent(agent string) bool {
	rel, err := skillDirForAgent(agent)
	if err != nil {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	for _, name := range []string{"browser", "check"} {
		if _, err := os.Stat(filepath.Join(home, rel, name, "SKILL.md")); err != nil {
			return false
		}
	}
	return true
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
