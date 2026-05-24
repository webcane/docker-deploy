---
phase: quick
plan: 260524-brw
type: execute
wave: 1
depends_on: []
files_modified:
  - /Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb
  - /Users/mniedre/git/docker-deploy/.goreleaser.yaml
autonomous: true
requirements: []

must_haves:
  truths:
    - "brew install docker-deploy completes without a symlink warning"
    - "The symlink at ~/.docker/cli-plugins/docker-deploy is created automatically by post_install"
    - "Future goreleaser releases regenerate the formula with sandbox_allowlist? present"
  artifacts:
    - path: "/Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb"
      provides: "Fixed formula with sandbox_allowlist? and improved caveats"
      contains: "sandbox_allowlist?"
    - path: "/Users/mniedre/git/docker-deploy/.goreleaser.yaml"
      provides: "Updated brews.custom_block so regenerated formula includes sandbox_allowlist?"
  key_links:
    - from: "post_install (formula)"
      to: "~/.docker/cli-plugins/docker-deploy"
      via: "File.symlink"
      pattern: "sandbox_allowlist"
---

<objective>
Fix the "Could not create symlink automatically" warning that appears when users run `brew install docker-deploy`.

Root cause: The formula's `post_install` block attempts `File.symlink` to `~/.docker/cli-plugins/` (outside Homebrew prefix), which Homebrew's macOS sandbox blocks — producing the EPERM rescue path that prints the warning. The fix requires adding `sandbox_allowlist? = true` to the formula class so Homebrew lifts the write restriction for the home directory during post_install.

Additionally, the generated formula lost the `sandbox_allowlist?` method that was present in `.goreleaser.yaml`'s `custom_block` — likely a GoReleaser rendering issue. Both the live formula and the goreleaser template need to be fixed.

Purpose: Users experience a clean install with the symlink created automatically and `docker deploy` discoverable by the Docker CLI immediately after install.
Output: Updated `docker-deploy.rb` in the tap repo and updated `.goreleaser.yaml` for future releases.
</objective>

<execution_context>
@/Users/mniedre/git/docker-deploy/.claude/get-shit-done/workflows/execute-plan.md
@/Users/mniedre/git/docker-deploy/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@/Users/mniedre/git/docker-deploy/.planning/PROJECT.md
@/Users/mniedre/git/docker-deploy/.planning/STATE.md

The tap repo lives at /Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb (separate git repo).
The goreleaser config lives at /Users/mniedre/git/docker-deploy/.goreleaser.yaml.

Current formula state: Has post_install with File.symlink + Errno::EPERM rescue, but is MISSING
`sandbox_allowlist? = true`. Without it, macOS Homebrew sandbox blocks writes outside Homebrew
prefix, causing EPERM and the warning message.

The goreleaser custom_block currently contains `sandbox_allowlist?` as a def method, but the
generated formula shows it was not rendered — the live formula needs a direct fix too.

Homebrew sandbox_allowlist reference:
  https://docs.brew.sh/Formula-Cookbook#allow-filesystem-access-outside-homebrew-directories
The formula class must declare `HOMEBREW_SANDBOX_ALLOWLIST_HOME = :write` OR define:
  def sandbox_allowlist? = true
This lifts the sandbox restriction so post_install can write to ~/.docker/cli-plugins/.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix live formula — add sandbox_allowlist and improve caveats</name>
  <files>/Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb</files>
  <action>
Edit /Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb to add two things:

1. Add `def sandbox_allowlist? = true` as a class-level method immediately before `def post_install`.
   This tells Homebrew's macOS sandbox to allow writes outside the Homebrew prefix (specifically to
   ~/.docker/cli-plugins/) during post_install execution. Without this the sandbox raises EPERM.

2. Update the `caveats` block to include the symlink command in case sandbox_allowlist does not
   apply on the user's system (older Homebrew, Linux):

   def caveats
     <<~EOS
       docker-deploy is installed as a Docker CLI plugin.

       If the symlink to ~/.docker/cli-plugins/docker-deploy was not created automatically, run:
         ln -sf #{opt_bin}/docker-deploy ~/.docker/cli-plugins/docker-deploy

       To remove the Docker CLI plugin symlink on uninstall:
         rm -f ~/.docker/cli-plugins/docker-deploy
     EOS
   end

The post_install block itself does NOT need changes — it already has the correct File.symlink logic
and Errno::EPERM rescue. The sandbox_allowlist method is the missing piece.

Preserve the "# typed: false" and "# frozen_string_literal: true" header, all URLs/sha256 values,
and all other formula content exactly as-is.
  </action>
  <verify>
    <automated>grep -n "sandbox_allowlist" /Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb</automated>
  </verify>
  <done>
    Formula contains `def sandbox_allowlist? = true` before `def post_install`.
    Caveats block includes the manual symlink command.
    All sha256 values and URLs unchanged.
  </done>
</task>

<task type="auto">
  <name>Task 2: Fix goreleaser template so future releases preserve sandbox_allowlist</name>
  <files>/Users/mniedre/git/docker-deploy/.goreleaser.yaml</files>
  <action>
The current .goreleaser.yaml has a `custom_block` that defines `sandbox_allowlist?` as a Ruby def
method, but uses the block form (`def sandbox_allowlist?` / `true` / `end`) which GoReleaser may
have failed to render correctly in the generated formula.

Replace the `custom_block` in the brews section with content that matches what actually needs to
appear in the formula. The block should contain:

    custom_block: |
      def sandbox_allowlist? = true

      def uninstall
        target = "#{Dir.home}/.docker/cli-plugins/docker-deploy"
        File.delete(target) if File.exist?(target)
      end

Also update the `post_install` block's `rescue` clause `opoo` message to use the caveats-friendly
phrasing (users are told to check caveats). Change:

    opoo "Could not create symlink automatically. Run manually:\n" \
         "  ln -sf #{opt_bin}/docker-deploy ~/.docker/cli-plugins/docker-deploy"

to:

    opoo "Could not create symlink to ~/.docker/cli-plugins/docker-deploy. " \
         "See `brew info docker-deploy` for manual instructions."

This keeps the warning short and points to caveats (which has the full command) rather than
duplicating it inline.
  </action>
  <verify>
    <automated>grep -A2 "sandbox_allowlist" /Users/mniedre/git/docker-deploy/.goreleaser.yaml</automated>
  </verify>
  <done>
    .goreleaser.yaml custom_block contains `def sandbox_allowlist? = true` on a single line.
    post_install opoo message updated to reference `brew info`.
    File is valid YAML (no syntax errors).
  </done>
</task>

<task type="auto">
  <name>Task 3: Commit fixes in both repos</name>
  <files>/Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb, /Users/mniedre/git/docker-deploy/.goreleaser.yaml</files>
  <action>
Commit in each repo separately.

Tap repo (/Users/mniedre/git/homebrew-docker-deploy/):
  git -C /Users/mniedre/git/homebrew-docker-deploy add docker-deploy.rb
  git -C /Users/mniedre/git/homebrew-docker-deploy commit -m "fix: add sandbox_allowlist and improve caveats for cli-plugins symlink"

Main repo (/Users/mniedre/git/docker-deploy/):
  git -C /Users/mniedre/git/docker-deploy add .goreleaser.yaml
  git -C /Users/mniedre/git/docker-deploy commit -m "fix(brew): goreleaser template — sandbox_allowlist and updated opoo message"

Also update STATE.md quick tasks table with this entry:
  | 260524-brw | Fix brew install symlink warning — add sandbox_allowlist? to formula and goreleaser template | 2026-05-24 | (commit sha) | [260524-brw-fix-brew-install-symlink](./quick/260524-brw-fix-brew-install-symlink/) |
  </action>
  <verify>
    <automated>git -C /Users/mniedre/git/homebrew-docker-deploy log --oneline -1 && git -C /Users/mniedre/git/docker-deploy log --oneline -1</automated>
  </verify>
  <done>
    Both repos have a clean commit.
    STATE.md quick tasks table updated with this task's entry.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| formula post_install -> user home dir | Brew formula writes a symlink to ~/.docker/cli-plugins/ — symlink target is controlled by Homebrew (opt_bin), not user input |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-brw-01 | Tampering | post_install symlink | accept | symlink target is `opt_bin/docker-deploy` — a Homebrew-managed path; no user input involved |
| T-brw-02 | Elevation of Privilege | sandbox_allowlist | accept | allowlist grants write to home dir only during post_install, not at runtime; Homebrew standard pattern for CLI tool plugins |
</threat_model>

<verification>
After executing:
1. `grep sandbox_allowlist /Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb` returns a match
2. `brew reinstall webcane/docker-deploy/docker-deploy` (on a test machine) completes without "Could not create symlink" warning
3. `ls -la ~/.docker/cli-plugins/docker-deploy` shows the symlink created automatically
4. `docker deploy` is discoverable: `docker deploy --help` works
</verification>

<success_criteria>
- Formula contains `def sandbox_allowlist? = true` before post_install
- Caveats block includes the manual `ln -sf` command as a fallback
- .goreleaser.yaml custom_block uses single-line method form for sandbox_allowlist
- Both repos committed
- STATE.md updated
</success_criteria>

<output>
After completion, create /Users/mniedre/git/docker-deploy/.planning/quick/260524-brw-fix-brew-install-symlink/260524-brw-SUMMARY.md
</output>
