# Use Vibium with Grok Build

Give Grok Build browser tools through Vibium's CLI skills. Grok Build loads
user skills from `~/.grok/skills/<name>/SKILL.md`. Use a local session with
shell access. The examples below use Bash or Zsh.

This guide assumes `vibium` is on your PATH. On Linux, prefix, EEXIST,
postinstall, and nightly installs are in
[Install the Vibium CLI on Linux](install-on-linux.md).

## 1. Install Vibium and the skills

```bash
npm install -g vibium
vibium add-skill --agent grok
vibium add-skill check --agent grok
vibium ready browser
```

Browser readiness checks installed files without opening a browser. Follow its
installation guidance if anything is missing.

Start a Grok Build session. Type `/` to find `browser` and `check`. Invoke
them as `/browser` and `/check`.

## 2. Ask Grok to use the browser

Send this as a Grok Build prompt:

```text
/browser Open https://example.com with Vibium, take a screenshot, and
summarize the page. Close the browser session you started afterward.
```

Grok reads the skill and runs Vibium commands. Direct browser commands need
no separate Vibium AI configuration.

If `vibium` is not found, give Grok the absolute path to the binary.

## 3. Add an independent check

Run and Check currently require the development build and
[Vibium AI configuration](../tutorials/first-check.md#1-configure-ai-access).
Your Grok login does not configure Vibium's model provider; you can use any
[supported provider](../reference/model-providers.md).

Create the settings file and edit it. Vibium loads `ai.env` at start when
the file is mode 0600:

```bash
vibium setup
# Or: vibium config init, then edit ~/.config/vibium/ai.env
vibium ready ai
```

Do not display the settings file or keys. Then send:

```text
/check Use Vibium to check whether https://var.parts is up and save
its evidence to sitecheck.zip. Run vibium ready ai first and
address setup errors. Do not display the settings file or keys.
Report the verdict and recording path.
```

For a full development workflow, follow
[Check with your coding agent](../tutorials/check-with-a-coding-agent.md).
