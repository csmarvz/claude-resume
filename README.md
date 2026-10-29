> [!WARNING]
> This is alpha software. It reads Claude Code's internal session files, whose format can change with any Claude Code update, and its delete feature removes files for good. It may break, show wrong data or delete more than you expect. Back up `~/.claude` before deleting sessions you care about.

# claude-resume
List all your claude sessions and resume them in the folder they were started in.

![claude-resume picker: moving through sessions with a preview, filtering, and deleting a session](demo/demo.gif)

## Install

macOS and Linux:

```
curl -fsSL https://raw.githubusercontent.com/csmarvz/claude-resume/main/install.sh | bash
```

Windows (PowerShell):

```
irm https://raw.githubusercontent.com/csmarvz/claude-resume/main/install.ps1 | iex
```

`claude-resume` is a single program with no dependencies. The installers download the build for your system from the latest GitHub release: on macOS and Linux into `~/.local/bin` (set `PREFIX` to install into `$PREFIX/bin` instead), on Windows into `%LOCALAPPDATA%\Programs\claude-resume`, which they add to your PATH. You can also download a binary from the [releases page](https://github.com/csmarvz/claude-resume/releases) yourself, or build it with Go 1.26 or later: `go install github.com/csmarvz/claude-resume@latest`.

Only the Linux build has been tested so far. The macOS and Windows builds are released with each version, but nobody has run them on a real machine yet. If you try one, please [open an issue](https://github.com/csmarvz/claude-resume/issues) to say whether it works.

The only other requirement is the `claude` CLI, to resume sessions. Updates use the system `curl`.

## Usage

```
claude-resume                  pick a session, or list the last 10 outside a terminal
claude-resume -a               list all sessions (--all)
claude-resume <id>             resume a session in its folder (a unique ID prefix is enough)
claude-resume -r <id> <title>  rename a session (--rename)
claude-resume -d <id>          delete a session after a confirmation (--delete)
claude-resume -d -y <id>       delete without asking; needs the full session ID (--yes)
claude-resume --archive <id>   archive a session so that it never expires
claude-resume --restore <id>   move an archived session back to the session list
claude-resume update           install the latest release
claude-resume -v               show the installed version (--version)
claude-resume -h               show help (--help)
```

The title is the name you gave the session with `/rename`, otherwise the title claude generated, otherwise the first prompt. Sessions are read from `$CLAUDE_CONFIG_DIR/projects`, or `~/.claude/projects` if that variable is unset.

You can:
* Resume sessions
* Preview sessions
* Search sessions
* Rename sessions
* Archive sessions
* Delete sessions

In the picker, type `/` to open the commands for the selected session: `/archive`, `/list-archives`, `/rename`, `/delete` and `/folders`. The `/` opens them when the search line is empty or after you move through the results with the arrow keys; while you type a search, it stays part of the search.

Renaming writes the same title record as `/rename` in Claude Code. Sessions that are open in a running Claude Code process are refused, because it could write its own title back.

## Archiving sessions

`/archive` moves a session's files into `~/.claude/claude-resume/archive`. The session leaves the list, and Claude Code's cleanup doesn't look in that folder, so it never expires. `/list-archives` shows the archived sessions: ENTER restores one and resumes it, `/restore` only restores it. A restored session gets today's date, so Claude Code's 30 days start again from the restore. Sessions that are open in a running Claude Code process are refused.

## Deleting sessions

A delete removes the session's transcript, its folder of subagent and tool output, and its `file-history` and `session-env` entries. It leaves `~/.claude/history.jsonl`, the prompt history behind the up arrow in Claude Code, untouched. Sessions that are open in a running Claude Code process are refused.

`-y` skips the dialog and works without a terminal, so you can script it. It only accepts a full session ID, never a prefix.

Claude Code also deletes old transcripts on its own: by default, sessions untouched for 30 days (the `cleanupPeriodDays` setting). Archive a session to keep it.

## Updating

Run `claude-resume update`. It downloads the latest release for your system, checks it against the release's SHA-256 checksums and replaces itself; if anything fails, the installed version stays as it was.

Once a day, the picker asks GitHub in the background whether a newer release is out, and shows it in yellow at the bottom right when there is one. Set `CLAUDE_RESUME_NO_UPDATE_CHECK=1` to turn that check off.

Version 0.0.1-alpha has no `update` command: to leave it, run the install command once more.

## Uninstall

```
rm "${PREFIX:-$HOME/.local}/bin/claude-resume"
```

On Windows, delete the `%LOCALAPPDATA%\Programs\claude-resume` folder and remove it from your PATH.

## Caveat

`claude-resume` reads Claude Code's session files directly. Their format is not a public API, so a Claude Code update can break it.

## License

MIT
