# claude-resume

Lists your Claude Code sessions from every project folder, newest first, and resumes one in the folder it was started in.

```
2026-10-25 17:35  Checkout total rounding fix    ~/dev/shop          3112af4c-2168-4178-b106-863032d0fe40
2026-10-24 00:25  Order export walkthrough       ~/dev/shop/backend  cfba6d38-db92-4f58-bb85-77d44402490a
2026-10-22 13:57  RSS feed                       ~/dev/blog          31cbcd4d-dcd3-4e3d-9268-1f4b756a0cbf
```

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

You also need the `claude` CLI to resume sessions.

## Usage

```
claude-resume         pick a session with fzf, or list the last 10 without it
claude-resume -a      list all sessions (--all)
claude-resume <id>    resume a session in its folder (a unique ID prefix is enough)
claude-resume -h      show help (--help)
```

In the picker, type to filter by folder, date or prompt, and press Enter to resume. When the output goes to a pipe or a file, the script prints the list instead of opening the picker.

Each line shows the last activity, title, working directory and session ID. The title is the name you gave the session with `/rename`, otherwise the title Claude Code generated, otherwise the first prompt. Sessions are read from `$CLAUDE_CONFIG_DIR/projects`, or `~/.claude/projects` if that variable is unset.

## Uninstall

```
rm ~/.local/bin/claude-resume
```

`jq` and `fzf` stay installed.

## Caveat

The script reads Claude Code's session files directly. Their format is not a public API, so a Claude Code update can break it.

## License

MIT
