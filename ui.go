package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

type tty struct {
	out  *bufio.Writer
	old  *term.State
	keys chan []byte
	wake chan struct{}
	pend []byte
}

func openTTY() *tty {
	old, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		die("cannot use the terminal: %v", err)
	}
	enableVT()
	t := &tty{out: bufio.NewWriterSize(os.Stdout, 64<<10), old: old, keys: make(chan []byte, 64), wake: make(chan struct{}, 1)}
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				close(t.keys)
				return
			}
			t.keys <- append([]byte(nil), buf[:n]...)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, os.Interrupt)
	go func() {
		<-sig
		t.close()
		os.Exit(143)
	}()
	t.out.WriteString("\x1b[?1049h")
	cleanup = t.close
	return t
}

func (t *tty) close() {
	t.out.WriteString("\x1b[?25h\x1b[?1049l")
	t.out.Flush()
	term.Restore(int(os.Stdin.Fd()), t.old)
	cleanup = func() {}
}

func (t *tty) size() (int, int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w < 1 || h < 1 {
		return 80, 24
	}
	return w, h
}

func (t *tty) drain() {
	t.pend = nil
	for {
		select {
		case <-t.keys:
		default:
			return
		}
	}
}

func (t *tty) poke() {
	select {
	case t.wake <- struct{}{}:
	default:
	}
}

var csi = map[string]string{
	"A": "up", "B": "down", "C": "right", "D": "left", "H": "home", "F": "end",
	"1;2A": "shift-up", "1;2B": "shift-down", "5~": "pgup", "6~": "pgdn", "3~": "del",
	"1~": "home", "4~": "end", "7~": "home", "8~": "end",
}

var ctrl = map[byte]string{
	0x00: "ctrl-space", 0x01: "ctrl-a", 0x02: "left", 0x03: "esc", 0x04: "ctrl-d", 0x05: "end", 0x06: "right",
	0x08: "bs", 0x7f: "bs", 0x0a: "down", 0x0b: "up", 0x0d: "enter", 0x0e: "down", 0x10: "up",
	0x15: "ctrl-u", 0x17: "ctrl-w",
}

// read returns the next key, or "" after a quiet interval or a wake-up so the caller can redraw.
func (t *tty) read() string {
	for len(t.pend) == 0 {
		select {
		case b, ok := <-t.keys:
			if !ok {
				return "esc"
			}
			t.pend = append(t.pend, b...)
		case <-t.wake:
			return ""
		case <-time.After(250 * time.Millisecond):
			return ""
		}
	}
	p := t.pend
	if p[0] == 0x1b {
		if len(p) == 1 {
			select {
			case b := <-t.keys:
				p = append(p, b...)
			case <-time.After(30 * time.Millisecond):
			}
		}
		if len(p) == 1 {
			t.pend = nil
			return "esc"
		}
		if p[1] != '[' && p[1] != 'O' {
			t.pend = p[2:]
			return ""
		}
		i := 2
		for i < len(p) && (p[i] < 0x40 || p[i] > 0x7e) {
			i++
		}
		if i == len(p) {
			t.pend = nil
			return ""
		}
		t.pend = p[i+1:]
		return csi[string(p[2:i+1])]
	}
	if p[0] < 0x20 || p[0] == 0x7f {
		t.pend = p[1:]
		return ctrl[p[0]]
	}
	r, n := utf8.DecodeRune(p)
	t.pend = p[n:]
	if r == utf8.RuneError {
		return ""
	}
	return string(r)
}

type view struct {
	prompt  string
	items   []string
	header  []string
	foot    string
	preview func(i int) []line
	keys    map[string]bool

	query     []rune
	cur       int
	sel, top  int
	dim, hide bool
	pscroll   int
	shown     []int
	pos       [][]int
}

func (v *view) filter() {
	terms := strings.Fields(string(v.query))
	v.shown, v.pos = v.shown[:0], v.pos[:0]
	for i, it := range v.items {
		if p, ok := match(it, terms); ok {
			v.shown = append(v.shown, i)
			v.pos = append(v.pos, p)
		}
	}
	v.sel = max(0, min(v.sel, len(v.shown)-1))
}

func (t *tty) run(v *view) (string, int) {
	v.filter()
	for {
		t.draw(v)
		k := t.read()
		n := len(v.shown)
		edited := false
		switch k {
		case "":
			continue
		case "esc":
			return k, -1
		case "ctrl-a":
			if v.keys[k] {
				return k, -1
			}
		case "enter", "ctrl-d":
			if n > 0 && (k == "enter" || v.keys[k]) {
				return k, v.shown[v.sel]
			}
		case "up":
			v.dim = v.sel > 0
			v.sel = max(0, v.sel-1)
			v.pscroll = 0
		case "down":
			v.dim = true
			v.sel = min(max(n-1, 0), v.sel+1)
			v.pscroll = 0
		case "pgup", "pgdn":
			_, h := t.size()
			step := max(1, h/3)
			if k == "pgup" {
				step = -step
			}
			v.sel = max(0, min(n-1, v.sel+step))
			v.pscroll = 0
		case "shift-up":
			v.pscroll = max(0, v.pscroll-1)
		case "shift-down":
			v.pscroll++
		case "ctrl-space":
			v.hide = !v.hide
		case "left":
			v.cur = max(0, v.cur-1)
		case "right":
			v.cur = min(len(v.query), v.cur+1)
		case "home":
			v.cur = 0
		case "end":
			v.cur = len(v.query)
		case "bs":
			if v.cur > 0 {
				v.query = append(v.query[:v.cur-1], v.query[v.cur:]...)
				v.cur--
				edited = true
			}
		case "del":
			if v.cur < len(v.query) {
				v.query = append(v.query[:v.cur], v.query[v.cur+1:]...)
				edited = true
			}
		case "ctrl-u":
			v.query = append([]rune(nil), v.query[v.cur:]...)
			v.cur, edited = 0, true
		case "ctrl-w":
			i := v.cur
			for i > 0 && v.query[i-1] == ' ' {
				i--
			}
			for i > 0 && v.query[i-1] != ' ' {
				i--
			}
			v.query = append(v.query[:i], v.query[v.cur:]...)
			v.cur, edited = i, true
		default:
			if utf8.RuneCountInString(k) == 1 {
				r := []rune(k)[0]
				v.query = append(v.query[:v.cur], append([]rune{r}, v.query[v.cur:]...)...)
				v.cur++
				edited = true
			}
		}
		if edited {
			v.dim, v.sel, v.top, v.pscroll = false, 0, 0, 0
			v.filter()
		}
	}
}

func clip(s string, w int) string {
	if width(s) > w {
		return cut(s, max(0, w-2)) + "··"
	}
	return s
}

func highlight(s string, pos []int, hl, normal string) string {
	var b strings.Builder
	j := 0
	for i, r := range []rune(s) {
		for j < len(pos) && pos[j] < i {
			j++
		}
		if j < len(pos) && pos[j] == i {
			b.WriteString(hl + string(r) + normal)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func wrap(lines []line, w int) []line {
	w = max(w, 3)
	var out []line
	for _, l := range lines {
		r := []rune(l.text)
		out = append(out, line{l.style, string(r[:min(len(r), w)])})
		for r = r[min(len(r), w):]; len(r) > 0; r = r[min(len(r), w-2):] {
			out = append(out, line{"cont" + l.style, string(r[:min(len(r), w-2)])})
		}
	}
	return out
}

func (t *tty) draw(v *view) {
	w, h := t.size()
	var rows []string
	p := "\x1b[1;38;5;110m" + v.prompt
	if v.dim {
		p = "\x1b[1;2;38;5;110m" + v.prompt
	}
	rows = append(rows, p+"\x1b[0;1m"+clip(string(v.query), max(0, w-width(v.prompt)))+"\x1b[0m")
	info := fmt.Sprintf("%d/%d", len(v.shown), len(v.items))
	rows = append(rows, "  \x1b[38;5;144m"+info+"\x1b[38;5;59m "+strings.Repeat("─", max(0, w-3-len(info)))+"\x1b[0m")
	for _, hd := range v.header {
		rows = append(rows, "  \x1b[38;5;109m"+clip(hd, w-2)+"\x1b[0m")
	}

	ph := 0
	if v.preview != nil && !v.hide && h >= 10 {
		ph = h * 60 / 100
	}
	area := max(1, h-len(rows)-ph-1)
	if v.sel < v.top {
		v.top = v.sel
	}
	if v.sel >= v.top+area {
		v.top = v.sel - area + 1
	}
	for i := v.top; i < v.top+area; i++ {
		if i >= len(v.shown) {
			rows = append(rows, "")
			continue
		}
		s := clip(v.items[v.shown[i]], w-2)
		if i == v.sel {
			rows = append(rows, "\x1b[1;38;5;161;48;5;236m▌\x1b[0;48;5;236m \x1b[1;38;5;254m"+
				highlight(s, v.pos[i], "\x1b[38;5;151m", "\x1b[38;5;254m")+strings.Repeat(" ", max(0, w-2-width(s)))+"\x1b[0m")
		} else {
			rows = append(rows, "\x1b[38;5;236m▌\x1b[0m "+highlight(s, v.pos[i], "\x1b[38;5;108m", "\x1b[39m"))
		}
	}

	if ph > 0 {
		inner := w - 4
		rows = append(rows, "\x1b[38;5;59m╭"+strings.Repeat("─", max(0, w-2))+"╮\x1b[0m")
		var body []line
		if len(v.shown) > 0 {
			body = wrap(v.preview(v.shown[v.sel]), inner)
		}
		v.pscroll = min(v.pscroll, max(0, len(body)-(ph-2)))
		for i := v.pscroll; i < v.pscroll+ph-2; i++ {
			s, style := "", ""
			if i < len(body) {
				s, style = body[i].text, body[i].style
			}
			pad := strings.Repeat(" ", max(0, inner-width(s)))
			if strings.HasPrefix(style, "cont") {
				style = style[4:]
				pad = pad[min(len(pad), 2):]
				s = "\x1b[2m↳ \x1b[0m" + "\x1b[" + style + "m" + s
			} else {
				s = "\x1b[" + style + "m" + s
			}
			rows = append(rows, "\x1b[38;5;59m│\x1b[0m "+s+"\x1b[0m"+pad+" \x1b[38;5;59m│\x1b[0m")
		}
		rows = append(rows, "\x1b[38;5;59m╰"+strings.Repeat("─", max(0, w-2))+"╯\x1b[0m")
	}

	b := t.out
	b.WriteString("\x1b[?25l")
	for i := 0; i < h; i++ {
		s := ""
		if i < len(rows) {
			s = rows[i]
		}
		if i == h-1 {
			s = "\x1b[38;5;145m" + clip(v.foot, w) + "\x1b[0m"
		}
		fmt.Fprintf(b, "\x1b[%d;1H%s\x1b[K", i+1, s)
	}
	fmt.Fprintf(b, "\x1b[1;%dH\x1b[?25h", min(w, width(v.prompt)+v.cur+1))
	b.Flush()
}

func folders(all []*session) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range all {
		if c := tilde(s.cwd); !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

func pick(all []*session) {
	t := openTTY()
	loadAll(all)
	var mu sync.Mutex
	cache := map[string][]line{}
	loading := map[string]bool{}
	var shown []*session
	v := &view{prompt: "search> ", keys: map[string]bool{"ctrl-a": true, "ctrl-d": true}}
	v.preview = func(i int) []line {
		s := shown[i]
		mu.Lock()
		defer mu.Unlock()
		if l, ok := cache[s.id]; ok {
			return l
		}
		if !loading[s.id] {
			loading[s.id] = true
			go func() {
				l := preview(s)
				mu.Lock()
				cache[s.id] = l
				mu.Unlock()
				t.poke()
			}()
		}
		return nil
	}
	dir, msg, choose, note := "", "", false, notice()
	for {
		if choose {
			f := &view{prompt: "folder> ", items: folders(all), foot: "  ↑↓ select · ENTER show its sessions · ESC all sessions"}
			k, i := t.run(f)
			dir = ""
			if k == "enter" {
				dir = f.items[i]
			}
			msg, choose, v.sel, v.top = "", false, 0, 0
		}
		shown = shown[:0]
		for _, s := range all {
			if dir == "" || tilde(s.cwd) == dir {
				shown = append(shown, s)
			}
		}
		if len(shown) == 0 {
			dir, shown = "", append(shown, all...)
		}
		v.items = rows(shown)
		v.header = nil
		if dir != "" {
			v.header = append(v.header, "folder: "+dir)
		}
		if msg != "" {
			v.header = append(v.header, msg)
		}
		if note != "" {
			v.header = append(v.header, note)
		}
		if len(v.header) == 0 {
			v.header = []string{""}
		}
		esc := "quit"
		if dir != "" {
			esc = "folders"
		}
		v.foot = "  Type to search · ↑↓ select · ENTER resume · CTRL-D delete · CTRL-SPACE preview · CTRL-A folders · ESC " + esc
		k, i := t.run(v)
		switch k {
		case "esc":
			if dir == "" {
				t.close()
				os.Exit(0)
			}
			choose = true
		case "ctrl-a":
			choose = true
		case "enter":
			t.close()
			resume(all, shown[i].id)
		case "ctrl-d":
			t.drain()
			var ok bool
			msg, ok = remove(all, shown[i].id, false, t)
			if ok {
				for j, s := range all {
					if s.id == shown[i].id {
						all = append(all[:j], all[j+1:]...)
						break
					}
				}
			}
			if len(all) == 0 {
				t.close()
				fmt.Println(msg)
				os.Exit(0)
			}
		}
	}
}

func (t *tty) confirm(lines []line) bool {
	t.drain()
	sel, last := 0, time.Now()
	for {
		t.dialog(lines, sel)
		switch t.read() {
		case "":
			if time.Since(last) > 10*time.Minute {
				return false
			}
			continue
		case "left":
			sel = 0
		case "right":
			sel = 1
		case "enter":
			return sel == 1
		case "esc", "q":
			return false
		}
		last = time.Now()
	}
}

func (t *tty) dialog(lines []line, sel int) {
	cols, rows := t.size()
	w := min(cols-4, 96)
	if w < 30 {
		w = min(cols, 30)
	}
	inner := w - 4
	avail := rows - 2
	tail := 1
	if avail >= 4 {
		tail = 2
	}
	pad := 0
	if avail-tail >= len(lines)+2 {
		pad = 1
	}
	n := max(0, min(avail-tail-2*pad, len(lines)))
	for n > 0 && lines[n-1].text == "" {
		n--
	}
	h := n + tail + 2*pad + 2
	top, left := 1, 1
	if rows > h {
		top = (rows-h)/2 + 1
	}
	if cols > w {
		left = (cols-w)/2 + 1
	}
	b := t.out
	r := top
	row := func(style, s string) {
		if width(s) > inner {
			s = cut(s, inner-1) + "…"
		}
		fmt.Fprintf(b, "\x1b[%d;%dH\x1b[31m│\x1b[0m \x1b[%sm%s\x1b[0m\x1b[%d;%dH\x1b[31m│\x1b[0m\x1b[K", r, left, style, s, r, left+w-1)
		r++
	}
	label, hint := " Delete session ", " ←→ select · ENTER choose · ESC cancel "
	if width(label) > w-4 {
		label = ""
	}
	if width(hint) > w-6 {
		hint = ""
	}
	b.WriteString("\x1b[?25l\x1b[2J")
	fmt.Fprintf(b, "\x1b[%d;%dH\x1b[31m╭──%s%s╮\x1b[0m", r, left, label, strings.Repeat("─", max(0, w-4-width(label))))
	r++
	if pad == 1 {
		row("0", "")
	}
	for _, l := range lines[:n] {
		row(l.style, l.text)
	}
	if tail == 2 {
		row("0", "")
	}
	b0, b1, gap := "[ CANCEL ]", "[ CONFIRM ]", "   "
	if inner < 24 {
		b0, b1, gap = "[CANCEL]", "[CONFIRM]", " "
	}
	lpad := max(0, (inner-len(b0)-len(gap)-len(b1))/2)
	fmt.Fprintf(b, "\x1b[%d;%dH\x1b[31m│\x1b[0m %*s", r, left, lpad, "")
	if sel == 0 {
		fmt.Fprintf(b, "\x1b[7m%s\x1b[0m%s\x1b[31m%s\x1b[0m", b0, gap, b1)
	} else {
		fmt.Fprintf(b, "%s%s\x1b[1;7;31m%s\x1b[0m", b0, gap, b1)
	}
	fmt.Fprintf(b, "\x1b[%d;%dH\x1b[31m│\x1b[0m\x1b[K", r, left+w-1)
	r++
	if pad == 1 {
		row("0", "")
	}
	fmt.Fprintf(b, "\x1b[%d;%dH\x1b[31m╰──%s%s╯\x1b[0m", r, left, hint, strings.Repeat("─", max(0, w-4-width(hint))))
	b.Flush()
}
