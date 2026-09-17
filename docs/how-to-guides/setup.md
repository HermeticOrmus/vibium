# Set up Vibium

`vibium setup` is the happy path after install. It configures the local
browser, writes `~/.config/vibium/ai.env`, and can install agent skills, then
runs [`vibium ready`](../reference/ready.md).

```bash
npm install -g vibium
vibium setup
```

On a terminal, setup asks for AI in that same run. Agents and CI with no TTY:

```bash
vibium setup --non-interactive
```

`--quick` fills only what is missing. Sections can run on their own:

```bash
vibium setup browser
vibium setup ai
vibium setup skills
```

## Browser

If the selected engine is missing, setup runs the same install path as
`vibium install`. If the files are already present, it says so. It then checks
those files the same way `vibium ready browser` does. It does not launch
Chrome or Firefox.

Use `--engine` and `--channel` for the same selection as live commands.

## AI

Setup prompts for provider, model, and (when needed) an API key. The key is
not echoed. openai-compatible and local also prompt for a base URL.

It writes `~/.config/vibium/ai.env` at mode 0600. If that file already exists,
the previous copy is kept as `ai.env.bak`. The setup process uses those values
immediately so the following `ready` check in the same run sees them. Later
commands load empty AI variables from that file automatically; see
[model providers](../reference/model-providers.md).

On a terminal, setup prompts for those values immediately. `--non-interactive`
skips prompts only when stdin is not a TTY, and does not overwrite an existing
`ai.env`. If there is no terminal and no file, that section is skipped. Re-run
`vibium setup` in a terminal to configure AI. `vibium config init` still writes
the commented template. Values are never printed.

## Skills

If `~/.grok` exists, setup installs the `browser` and `check` skills under
`~/.grok/skills`; otherwise `~/.claude/skills`. Non-interactive setup
installs skills only when one of those directories already exists.

## Readiness

Setup finishes by running the matching `vibium ready` checks (files only for
the browser; up to two model requests for AI when a key is present). Use
`--json` for one `{ok, result}` envelope with section statuses and paths.
