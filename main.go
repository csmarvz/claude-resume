package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/term"
)

const usageText = `usage: claude-resume              pick a session, or list the last 10 outside a terminal
       claude-resume -a           list all sessions (--all)
       claude-resume <id>         resume a session in its folder (a unique ID prefix is enough)
       claude-resume -d [-y] <id> delete a session for good; asks first unless -y (--delete, --yes)
`

var (
	home     string
	config   string
	projects string
	cleanup  = func() {}

	customMark     = []byte(`"type":"custom-title"`)
	aiMark         = []byte(`"type":"ai-title"`)
	userMark       = []byte(`"type":"user"`)
	assistantMark  = []byte(`"type":"assistant"`)
	toolResultMark = []byte(`"type":"tool_result"`)
)

type session struct {
	path, id   string
	mod        time.Time
	cwd, title string
	loaded     bool
}

type record struct {
	Type             string `json:"type"`
	Cwd              string `json:"cwd"`
	CustomTitle      string `json:"customTitle"`
	AiTitle          string `json:"aiTitle"`
	IsMeta           bool   `json:"isMeta"`
	IsCompactSummary bool   `json:"isCompactSummary"`
	Message          struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type line struct{ style, text string }

func isHex(s string) bool { return s != "" && strings.Trim(s, "0123456789abcdef-") == "" }

func isUUID(s string) bool {
	return len(s) == 36 && isHex(s) && strings.Count(s, "-") == 4 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}

func squeeze(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !space {
				b.WriteByte(' ')
			}
			space = true
			continue
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

func usage(code int) {
	fmt.Fprint(os.Stderr, usageText)
	os.Exit(code)
}

func die(format string, a ...any) {
	cleanup()
	fmt.Fprintf(os.Stderr, "claude-resume: "+format+"\n", a...)
	os.Exit(1)
}

func main() {
	args := append(os.Args[1:], "")
	arg, target, yes := args[0], "", false
	switch {
	case arg == "-h" || arg == "--help":
		usage(0)
	case arg == "" || arg == "-a" || arg == "--all":
	case arg == "-d" || arg == "--delete":
		for _, a := range args[1 : len(args)-1] {
			if a == "-y" || a == "--yes" {
				yes = true
				continue
			}
			if target != "" {
				usage(1)
			}
			target = a
		}
		if !isHex(target) {
			usage(1)
		}
		if yes && !isUUID(target) {
			die("-y needs the full session ID")
		}
	case strings.HasPrefix(arg, "-") || !isHex(arg):
		usage(1)
	}

	home, _ = os.UserHomeDir()
	config = os.Getenv("CLAUDE_CONFIG_DIR")
	if config == "" {
		config = filepath.Join(home, ".claude")
	}
	shown := filepath.Join(config, "projects")
	if abs, err := filepath.Abs(config); err == nil {
		config = abs
	}
	if st, err := os.Stat(config); err != nil || !st.IsDir() {
		die("no sessions found in %s", shown)
	}
	projects = filepath.Join(config, "projects")
	all := scan()
	if len(all) == 0 {
		die("no sessions found in %s", projects)
	}

	stdin, stdout := term.IsTerminal(int(os.Stdin.Fd())), term.IsTerminal(int(os.Stdout.Fd()))
	switch arg {
	case "-a", "--all":
		list(all, len(all))
	case "-d", "--delete":
		if !yes && !stdin {
			die("deleting needs a terminal to confirm; add -y to skip the confirmation")
		}
		var t *tty
		if !yes {
			t = openTTY()
		}
		msg, ok := remove(all, target, yes, t)
		if t != nil {
			t.close()
		}
		if !ok {
			die("%s", msg)
		}
		fmt.Println(msg)
	case "":
		if stdin && stdout {
			pick(all)
		} else {
			list(all, 10)
		}
	default:
		resume(all, arg)
	}
}

func scan() []*session {
	var list []*session
	dirs, _ := os.ReadDir(projects)
	for _, d := range dirs {
		files, _ := os.ReadDir(filepath.Join(projects, d.Name()))
		for _, f := range files {
			name := f.Name()
			id := strings.TrimSuffix(name, ".jsonl")
			if ok, _ := filepath.Match("????????-????-????-????-????????????.jsonl", name); !ok || !f.Type().IsRegular() {
				continue
			}
			info, err := f.Info()
			if err != nil {
				continue
			}
			list = append(list, &session{path: filepath.Join(projects, d.Name(), name), id: id, mod: info.ModTime()})
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].mod.After(list[j].mod) })
	return list
}

func eachLine(path string, want func(n int, head []byte) bool, fn func(n int, line []byte) bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 64<<10)
	for n := 0; ; n++ {
		chunk, err := r.ReadSlice('\n')
		if len(chunk) == 0 && err != nil {
			return
		}
		keep := want(n, chunk)
		line := chunk
		if err == bufio.ErrBufferFull {
			var full []byte
			if keep {
				full = append(full, chunk...)
			}
			for err == bufio.ErrBufferFull {
				chunk, err = r.ReadSlice('\n')
				if keep {
					full = append(full, chunk...)
				}
			}
			line = full
		}
		if keep && !fn(n, line) {
			return
		}
		if err != nil {
			return
		}
	}
}

func text(raw json.RawMessage, sep string) (string, bool) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, true
	}
	var parts []struct{ Type, Text string }
	if json.Unmarshal(raw, &parts) != nil {
		return "", false
	}
	var out []string
	for _, p := range parts {
		if p.Type == "tool_result" {
			return "", false
		}
		if p.Type == "text" {
			out = append(out, p.Text)
		}
	}
	return strings.Join(out, sep), true
}

func prompt(r *record) string {
	if r.Type != "user" {
		return ""
	}
	t, ok := text(r.Message.Content, " ")
	if !ok || strings.HasPrefix(t, "<") {
		return ""
	}
	return t
}

func (s *session) load() {
	var custom, ai, first, cwd string
	eachLine(s.path, func(n int, head []byte) bool {
		return n < 100 || bytes.Contains(head, customMark) || bytes.Contains(head, aiMark)
	}, func(n int, b []byte) bool {
		var r record
		json.Unmarshal(b, &r)
		if r.CustomTitle != "" {
			custom = r.CustomTitle
		}
		if r.AiTitle != "" {
			ai = r.AiTitle
		}
		if n < 100 {
			if cwd == "" {
				cwd = r.Cwd
			}
			if first == "" {
				first = prompt(&r)
			}
		}
		return true
	})
	if cwd == "" {
		cwd = "?"
	}
	title := custom
	if title == "" {
		title = ai
	}
	if title == "" {
		title = first
	}
	s.cwd, s.title, s.loaded = cwd, squeeze(title), true
}

func loadAll(list []*session) {
	ch := make(chan *session)
	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU()*2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range ch {
				s.load()
			}
		}()
	}
	for _, s := range list {
		if !s.loaded {
			ch <- s
		}
	}
	close(ch)
	wg.Wait()
}

func find(all []*session, prefix string) (*session, error) {
	var m []*session
	for _, s := range all {
		if strings.HasPrefix(s.id, prefix) {
			m = append(m, s)
		}
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("no session %s", prefix)
	}
	for _, s := range m {
		if s.id != m[0].id {
			return nil, fmt.Errorf("%s matches %d sessions, use more of the ID", prefix, len(m))
		}
	}
	return m[0], nil
}

func tilde(p string) string {
	if home != "" && (p == home || strings.HasPrefix(p, home+string(filepath.Separator))) {
		return "~" + p[len(home):]
	}
	return p
}

func date(s *session) string { return s.mod.Format("2006-01-02 15:04") }

func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func width(s string) int { return len([]rune(s)) }

func rows(list []*session) []string {
	tw, cw := 0, 0
	for _, s := range list {
		tw = max(tw, width(cut(s.title, 50)))
		cw = max(cw, width(tilde(s.cwd)))
	}
	out := make([]string, len(list))
	for i, s := range list {
		t, c := cut(s.title, 50), tilde(s.cwd)
		out[i] = fmt.Sprintf("%s  %s%*s  %s%*s  %s", date(s), t, tw-width(t), "", c, cw-width(c), "", s.id)
	}
	return out
}

func list(all []*session, n int) {
	sub := all[:min(n, len(all))]
	loadAll(sub)
	w := bufio.NewWriter(os.Stdout)
	for _, r := range rows(sub) {
		fmt.Fprintln(w, r)
	}
	w.Flush()
}

func preview(s *session) []line {
	reply := ""
	var prompts []string
	seen := map[string]bool{}
	eachLine(s.path, func(int, []byte) bool { return true }, func(_ int, b []byte) bool {
		isUser := bytes.Contains(b, userMark) && !bytes.Contains(b, toolResultMark)
		if !isUser && !bytes.Contains(b, assistantMark) {
			return true
		}
		var r record
		json.Unmarshal(b, &r)
		if r.Type == "assistant" && bytes.HasPrefix(r.Message.Content, []byte("[")) {
			if t, _ := text(r.Message.Content, "\n"); t != "" {
				reply = t
			}
		}
		if isUser && !r.IsMeta && !r.IsCompactSummary {
			if p := cut(squeeze(prompt(&r)), 300); p != "" && !seen[p] {
				seen[p] = true
				prompts = append(prompts, p)
			}
		}
		return true
	})
	title := s.title
	if title == "" {
		title = "(untitled)"
	}
	if reply == "" {
		reply = "(none)"
	}
	out := []line{{"1", title}, {"2", tilde(s.cwd) + "  ·  " + date(s) + "  ·  " + s.id}, {}, {"1", "Last reply"}}
	for _, l := range strings.Split(cut(reply, 1500), "\n") {
		out = append(out, line{"", l})
	}
	out = append(out, line{}, line{"1", "Prompts"})
	if len(prompts) == 0 {
		out = append(out, line{"", "(none)"})
	}
	for _, p := range prompts {
		out = append(out, line{"", "› " + p})
	}
	return out
}

func openPid(id string) string {
	files, _ := filepath.Glob(filepath.Join(config, "sessions", "*.json"))
	for _, f := range files {
		var s struct{ SessionId string }
		b, _ := os.ReadFile(f)
		if json.Unmarshal(b, &s) != nil || s.SessionId != id {
			continue
		}
		pid := strings.TrimSuffix(filepath.Base(f), ".json")
		var n int
		if _, err := fmt.Sscan(pid, &n); err == nil && isClaude(n) {
			return pid
		}
	}
	return ""
}

func size(p string) int64 {
	var n int64
	filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
		if err == nil {
			if i, err := d.Info(); err == nil && !d.IsDir() {
				n += i.Size()
			}
		}
		return nil
	})
	return n
}

func human(n int64) string {
	f, i := float64(n), 0
	for f >= 1024 && i < 4 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprint(n)
	}
	if f < 10 {
		return fmt.Sprintf("%.1f%c", math.Ceil(f*10)/10, "KMGT"[i-1])
	}
	return fmt.Sprintf("%.0f%c", math.Ceil(f), "KMGT"[i-1])
}

func remove(all []*session, prefix string, yes bool, t *tty) (string, bool) {
	s, err := find(all, prefix)
	if err != nil {
		return err.Error(), false
	}
	if !isUUID(s.id) {
		return "Not deleted: unexpected session file " + s.path, false
	}
	if pid := openPid(s.id); pid != "" {
		return fmt.Sprintf("Not deleted: session %s is open in Claude Code (pid %s). Close it first.", s.id[:8], pid), false
	}
	if !s.loaded {
		s.load()
	}
	var paths []string
	for _, p := range []string{s.path, strings.TrimSuffix(s.path, ".jsonl"), filepath.Join(config, "file-history", s.id), filepath.Join(config, "session-env", s.id)} {
		if _, err := os.Lstat(p); err == nil {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return "Not deleted: no files found for session " + s.id, false
	}
	name := s.title
	if name == "" {
		name = s.id
	}
	if !yes {
		title := s.title
		if title == "" {
			title = "(untitled)"
		}
		lines := []line{
			{"1;31", "WARNING: this permanently deletes the session below. It cannot be undone."}, {},
			{"1", title}, {"2", tilde(s.cwd) + "  ·  " + date(s) + "  ·  " + s.id}, {},
			{"", "Files to delete in " + tilde(config) + ":"},
		}
		for _, p := range paths {
			rel, _ := filepath.Rel(config, p)
			lines = append(lines, line{"", fmt.Sprintf("  %6s  %s", human(size(p)), rel)})
		}
		if !t.confirm(lines) {
			return "Cancelled, nothing was deleted.", false
		}
	}
	if pid := openPid(s.id); pid != "" {
		return fmt.Sprintf("Not deleted: session %s was opened in Claude Code (pid %s). Close it first.", s.id[:8], pid), false
	}
	failed := false
	for _, p := range paths {
		if err := os.RemoveAll(p); err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
		}
	}
	if failed {
		return "Only partly deleted \"" + name + "\", see the errors above.", false
	}
	return "Deleted \"" + name + "\".", true
}

func resume(all []*session, prefix string) {
	s, err := find(all, prefix)
	if err != nil {
		die("%s", err)
	}
	cwd := ""
	eachLine(s.path, func(n int, _ []byte) bool { return n < 100 }, func(_ int, b []byte) bool {
		var r record
		json.Unmarshal(b, &r)
		cwd = r.Cwd
		return cwd == ""
	})
	if cwd == "" {
		die("no folder recorded for session %s", prefix)
	}
	if os.Chdir(cwd) != nil {
		die("folder %s no longer exists", cwd)
	}
	execClaude(s.id)
}
