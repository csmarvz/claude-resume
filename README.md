> [!WARNING]
> This is alpha software. It reads Claude Code's internal session files, whose format can change with any Claude Code update, and its delete feature removes files for good. It may break, show wrong data or delete more than you expect. Back up `~/.claude` before deleting sessions you care about.

# claude-resume
List all your Claude Code sessions and resume them in the folder they were started in.
![claude-resume picker: moving through sessions with a preview, filtering, and deleting a session](demo/demo.gif)

## Install

```
curl -fsSL https://raw.githubusercontent.com/csmarvz/claude-resume/main/install.sh | bash
```

The installer puts `claude-resume` in `~/.local/bin` and installs `jq` and `fzf` if they are missing. It finds the package manager on its own (apt, dnf, pacman, zypper, apk or Homebrew) and uses `sudo` for everything except Homebrew. To install somewhere else, set `PREFIX` to a folder you can write to; the script goes in `$PREFIX/bin`.

To install from a clone instead:

```
git clone https://github.com/csmarvz/claude-resume.git
cd claude-resume
./install.sh
```

Requirements: bash (3.2 or later, so the macOS default works), `jq`, and the `claude` CLI to resume sessions. `fzf` is optional; the picker needs fzf 0.53 or later for the folder filter and its current layout, and falls back to a simpler version with older releases.

## Updating

Run the install command again; it replaces the installed script with the latest version:

```
curl -fsSL https://raw.githubusercontent.com/csmarvz/claude-resume/main/install.sh | bash
```

If you installed from a clone, run `git pull` and then `./install.sh` in it. The installer copies the script, so pulling alone does not change the installed copy.

### Updating fzf

The installer only installs `fzf` when it is missing and never upgrades it, and distribution packages are often older than 0.53 (Ubuntu 24.04 ships 0.44). Check your version with `fzf --version`. To get a current release:

- Homebrew (macOS or Linux): `brew install fzf`, or `brew upgrade fzf` if Homebrew already installed it.
- Any system with git: `git clone --depth 1 https://github.com/junegunn/fzf.git ~/.fzf && ~/.fzf/install`. The fzf installer downloads the binary and asks before it edits your shell profile.

## Usage

```
claude-resume              pick a session with fzf, or list the last 10 without it
claude-resume -a           list all sessions (--all)
claude-resume <id>         resume a session in its folder (a unique ID prefix is enough)
claude-resume -d <id>      delete a session after a confirmation (--delete)
claude-resume -d -y <id>   delete without asking; needs the full session ID (--yes)
claude-resume -h           show help (--help)
```

The picker opens when `fzf` is installed and the script runs in a terminal. Type to filter by date, title, folder or ID, move with the arrow keys, and press Enter to resume. The pane below the list shows the highlighted session's last reply and every prompt you sent in it; scroll it with Shift+Up and Shift+Down. Without `fzf`, or when input or output is a pipe or a file, the script prints the last 10 sessions instead; add `-a` for all of them.

Each line shows the last activity, title, working directory and session ID. The title is the name you gave the session with `/rename`, otherwise the title Claude Code generated, otherwise the first prompt. Sessions are read from `$CLAUDE_CONFIG_DIR/projects`, or `~/.claude/projects` if that variable is unset.

## Deleting sessions

Press Ctrl-D in the picker, or run `claude-resume -d <id>`. A dialog shows the session and every path that will be removed, with CANCEL selected. Use the left and right arrows to switch to CONFIRM and press Enter; Esc or Ctrl-C cancels. In the picker you then land back on the same search and row.

A delete removes the session's transcript, its folder of subagent and tool output, and its `file-history` and `session-env` entries. It leaves `~/.claude/history.jsonl`, the prompt history behind the up arrow in Claude Code, untouched. Sessions that are open in a running Claude Code process are refused.

`-y` skips the dialog and works without a terminal, so you can script it. It only accepts a full session ID, never a prefix.

Claude Code also deletes old transcripts on its own: by default, sessions untouched for 30 days (the `cleanupPeriodDays` setting).

## Uninstall

```
rm "${PREFIX:-$HOME/.local}/bin/claude-resume"
```

`jq` and `fzf` stay installed.

## Caveat

The script reads Claude Code's session files directly. Their format is not a public API, so a Claude Code update can break it.

## License

MIT
