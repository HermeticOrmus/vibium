# Set up Vibium

`vibium setup` is the happy path after install. It configures the local
browser, writes `~/.config/vibium/ai.env`, and can install agent skills, then
runs [`vibium ready`](../reference/ready.md).

```bash
npm install -g vibium
vibium setup
```

Agents and CI can skip prompts:

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
the previous copy is kept as `ai.env.bak`. Vibium loads the file at process
start when it is mode 0600, so you do not need to `source` it. An explicit
shell variable still wins over the file.

`--non-interactive` does not prompt and does not overwrite an existing
`ai.env`. If the file is missing, that section is skipped. Run `vibium setup ai`
in a terminal, or `vibium config init` to write the commented template.

A group- or world-readable `ai.env` is not loaded. `chmod 0600` the file and
rerun. Values are never printed.

## Skills

If `~/.grok` exists, setup defaults to `--agent grok`; otherwise claude. It
offers to install the `browser` and `check` skills through the same path as
`vibium add-skill`. Non-interactive setup installs skills only when
`~/.grok` or `~/.claude` already exists.

## Readiness

Setup finishes by running the matching `vibium ready` checks (files only for
the browser; up to two model requests for AI when a key is present). Use
`--json` for one `{ok, result}` envelope with section statuses and paths.
