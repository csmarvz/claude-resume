Output "/tmp/screenshot.gif"

Set Shell bash
Set FontSize 15
Set Width 1240
Set Height 560
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

Hide
Type "bash demo/sessions.sh /tmp/demo && export HOME=/home/demo CLAUDE_CONFIG_DIR=/tmp/demo PS1='$ ' && alias cr=$PWD/claude-resume && clear"
Enter
Sleep 1s
Show

Type "cr"
Sleep 300ms
Enter
Sleep 2s
Down
Sleep 1500ms
Up
Sleep 1500ms
Screenshot demo/screenshot.png
Sleep 500ms
