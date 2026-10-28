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

The only other requirement is the `claude` CLI, to resume sessions.

## Usage

```
claude-resume              pick a session, or list the last 10 outside a terminal
claude-resume -a           list all sessions (--all)
claude-resume <id>         resume a session in its folder (a unique ID prefix is enough)
claude-resume -d <id>      delete a session after a confirmation (--delete)
claude-resume -d -y <id>   delete without asking; needs the full session ID (--yes)
claude-resume -h           show help (--help)
```

The title is the name you gave the session with `/rename`, otherwise the title claude generated, otherwise the first prompt. Sessions are read from `$CLAUDE_CONFIG_DIR/projects`, or `~/.claude/projects` if that variable is unset.

You can:
* Resume sessions
* Preview sessions
* Search sessions
* Delete sessions
* TODO Rename sessions

## Deleting sessions

A delete removes the session's transcript, its folder of subagent and tool output, and its `file-history` and `session-env` entries. It leaves `~/.claude/history.jsonl`, the prompt history behind the up arrow in Claude Code, untouched. Sessions that are open in a running Claude Code process are refused.

`-y` skips the dialog and works without a terminal, so you can script it. It only accepts a full session ID, never a prefix.

Claude Code also deletes old transcripts on its own: by default, sessions untouched for 30 days (the `cleanupPeriodDays` setting).

## Updating

Run the install command again; it replaces the installed program with the latest release. If you installed with `go install`, run that again.

### Upgrading from the bash version

The first versions were a bash script that needed `jq` and `fzf`. To switch to the new program:

1. Run the install command above again. If you installed with a custom `PREFIX`, set the same one. The new program replaces the script at the same path, even when that path is a symlink into a clone of this repository, so aliases such as `cr` keep working.
2. Check that it worked: `claude-resume -h` should no longer mention fzf.
3. Optionally remove `jq` and `fzf` if nothing else uses them, for example with `sudo apt remove jq fzf` or `brew uninstall jq fzf`.

## Uninstall

```
rm "${PREFIX:-$HOME/.local}/bin/claude-resume"
```

On Windows, delete the `%LOCALAPPDATA%\Programs\claude-resume` folder and remove it from your PATH.

## Caveat

`claude-resume` reads Claude Code's session files directly. Their format is not a public API, so a Claude Code update can break it.

## License

MIT
