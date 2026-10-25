# claude-sessions

Lists your Claude Code sessions from every project folder, newest first, and resumes one in the folder it was started in.

```
claude-sessions         pick a session with fzf (if installed), else list the last 10
claude-sessions -a      list all sessions (--all)
claude-sessions <n>     resume session n in its folder
```

Each line shows the number, last activity, working directory, session ID and the first prompt (or the session summary, when there is one).

## Install

```
curl -fsSL https://raw.githubusercontent.com/csmarvz/claude-sessions/main/claude-sessions -o ~/.local/bin/claude-sessions
chmod +x ~/.local/bin/claude-sessions
```

Needs `jq` and bash. `fzf` is optional; without it you get the numbered list. Sessions are read from `$CLAUDE_CONFIG_DIR/projects`, or `~/.claude/projects` if that variable is unset.

The script reads Claude Code's session files directly. Their format is not a public API, so a Claude Code update can break it.
