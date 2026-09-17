# Install the Vibium CLI on Linux

Install `vibium` for a user account. Do not use sudo. This is the operator
recipe. Contributor VM setup is in
[Setting up local Vibium dev (x86 Linux)](../contributing/local-dev-setup-x86-linux.md).

## 1. Use a home prefix when npm's prefix is outside $HOME

Check where npm writes global binaries:

```bash
npm config get prefix
```

If that path is not under `$HOME` (Arch and Omarchy often use `/usr`), install
into your home directory and put that bin dir on PATH:

```bash
npm install -g --prefix ~/.local vibium
export PATH="$HOME/.local/bin:$PATH"
```

Keep `~/.local/bin` on PATH in your shell profile so later sessions find
`vibium`.

## 2. Move an existing `~/.local/bin/vibium` on EEXIST

If `~/.local/bin/vibium` already exists, npm can fail with `EEXIST`. Move or
rename the existing file, then install:

```bash
mv ~/.local/bin/vibium ~/.local/bin/vibium.bak
npm install -g --prefix ~/.local vibium
```

## 3. Run postinstall if npm skipped scripts

npm `ignore-scripts` or a client `allowScripts` block leaves `bin/cli.js` as
the Node shim. The Go binary is not linked over it, and Chrome for Testing is
not downloaded.

Run postinstall yourself (adjust the prefix if yours is not `~/.local`):

```bash
node ~/.local/lib/node_modules/vibium/postinstall.js
```

That replaces the shim with the Go binary and downloads Chrome for Testing.

## 4. Install a GitHub nightly when the npm tag is not what you want

`npm install vibium@nightly` tracks the npm `nightly` dist-tag. To pin a
specific GitHub nightly instead, install the JS tarball and the matching
platform tarball together.

Example tag: `nightly-2026.9.16-dev.20260916202545-a94e2e8`

Download from that GitHub release:

- `vibium-2026.9.16-dev.20260916202545.tgz`
- `vibium-linux-x64-2026.9.16-dev.20260916202545.tgz` (or `linux-arm64` on ARM)

Do not install Darwin tarballs on Linux.

The release `SHA256SUMS` prefixes paths with `release/npm/`. Verify by
basename:

```bash
grep 'vibium-2026.9.16-dev.20260916202545.tgz$' SHA256SUMS
sha256sum vibium-2026.9.16-dev.20260916202545.tgz
grep 'vibium-linux-x64-2026.9.16-dev.20260916202545.tgz$' SHA256SUMS
sha256sum vibium-linux-x64-2026.9.16-dev.20260916202545.tgz
```

Then install both tarballs in one command:

```bash
npm install -g --prefix ~/.local \
  ./vibium-2026.9.16-dev.20260916202545.tgz \
  ./vibium-linux-x64-2026.9.16-dev.20260916202545.tgz
```

Alternative: put the `vibium-linux-amd64` binary from the same release on
PATH (use `vibium-linux-arm64` on ARM). Then:

```bash
vibium install
vibium ready browser
```

## 5. AI settings

`vibium setup` writes `~/.config/vibium/ai.env`. Vibium loads that file at
start when it is mode 0600. For Run and Check:

```bash
vibium setup
# Or: vibium config init, then edit ~/.config/vibium/ai.env
vibium ready ai
```
