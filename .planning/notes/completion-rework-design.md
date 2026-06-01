---
name: completion-rework-design
description: Key design decisions for phase 10 completion rework — static cobra generation, hidden subcommand, CI-committed files, homebrew install path
metadata:
  type: project
---

## Shell Completion Rework — Design Decisions

### What changes from the original phase 10

Original phase 10 implemented **dynamic completions** — the binary read `deploy.yaml` and
`~/.ssh/config` at completion time to suggest `--host`, `--path`, and `--compose-file` values.

The rework replaces this with **static cobra-generated completions**:

- No runtime reads — cobra generates a static script from the command tree
- Completion files are generated once in CI and committed to the repo
- Users install the pre-generated file; the binary does not participate at completion time

### The `completion` subcommand

- Registered as a hidden cobra command (`Hidden: true`, `DisableFlagsInUseLine: true`)
- Does **not** appear in `--help` output
- Not mentioned in README or INSTALL.md
- Used only by the CI pipeline to generate the committed completion files

### CI generation (GitHub Actions)

On each release:
1. Build the binary
2. Run `docker-deploy completion zsh > contrib/_docker-deploy`
3. Run `docker-deploy completion bash > contrib/docker-deploy.bash`
4. Commit/include these files in the release tarball

### Homebrew install path

Use `(share/"zsh/site-functions").install "_docker-deploy"` in the formula.

**Why:** Homebrew installs the file to `Cellar/docker-deploy/<version>/share/zsh/site-functions/`
and creates a symlink at `/opt/homebrew/share/zsh/site-functions/_docker-deploy`, which is
already in `$FPATH` on homebrew-managed macOS. Zero user configuration required.

**Not** a custom directory like `share/zsh-docker-deploy-completion/` — that path is not in
`$FPATH` by default and would require manual fpath setup.

### Manual install fallback

A shell script that downloads the completion file from the release tarball and symlinks it to
the appropriate location (`/opt/homebrew/share/zsh/site-functions/` or `~/.zsh/completions/`).

### Documentation

INSTALL.md describes how to enable completions after install, without mentioning the hidden
subcommand. README.md does not mention completions.

**Why:** [completion-rework-design] — phase 10 is reopened to implement this approach.
