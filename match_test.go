package main

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestIDs(t *testing.T) {
	if !isUUID("3bf338a9-00db-4432-a914-59330a072b29") || isUUID("3bf338a9") || isUUID("3bf338a900db-4432-a914-59330a072b29-") {
		t.Error("isUUID")
	}
	if !isHex("3bf3") || isHex("") || isHex("zz") || squeeze(" a \t\n b ") != " a b " {
		t.Error("isHex or squeeze")
	}
}

func TestMatch(t *testing.T) {
	for _, c := range []struct {
		s, q string
		pos  []int
		ok   bool
	}{
		{"~/dev/shop/backend", "shop", []int{6, 7, 8, 9}, true},
		{"Order export", "oe", []int{0, 3}, true},
		{"Order export", "Oe", []int{0, 3}, true},
		{"order export", "Oe", nil, false},
		{"rss feed", "feed rss", []int{0, 1, 2, 4, 5, 6, 7}, true},
		{"rss feed", "rss x", nil, false},
		{"anything", "", nil, true},
	} {
		pos, ok := match(c.s, strings.Fields(c.q))
		if ok != c.ok || !reflect.DeepEqual(pos, c.pos) {
			t.Errorf("match(%q, %q) = %v, %v; want %v, %v", c.s, c.q, pos, ok, c.pos, c.ok)
		}
	}
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.0.5-alpha", "0.0.4-alpha", true},
		{"0.0.4-alpha", "0.0.5-alpha", false},
		{"0.0.4-alpha", "0.0.4-alpha", false},
		{"0.0.10-alpha", "0.0.9-alpha", true},
		{"0.1.0", "0.1.0-alpha", true},
		{"0.1.0-alpha", "0.1.0", false},
		{"0.0.4-alpha", "0.0.4-alpha.0.20261028142944-0be9857629ba", false},
		{"1.0.0", "0.9.9", true},
	} {
		if got := newer(c.a, c.b); got != c.want {
			t.Errorf("newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestWidth(t *testing.T) {
	for _, c := range []struct {
		s    string
		w    int
		cut4 string
	}{
		{"abcdef", 6, "abcd"},
		{"日本語です", 10, "日本"},
		{"a日b", 4, "a日b"},
		{"🚀go", 4, "🚀go"},
		{"éte", 3, "éte"},
		{"✔️ok", 3, "✔️ok"},
		{"\x1b[1mab\x1b[22mcdef", 6, "\x1b[1mab\x1b[22mcd"},
	} {
		if got := width(c.s); got != c.w {
			t.Errorf("width(%q) = %d, want %d", c.s, got, c.w)
		}
		if got := cut(c.s, 4); got != c.cut4 {
			t.Errorf("cut(%q, 4) = %q, want %q", c.s, got, c.cut4)
		}
	}
}

func TestMove(t *testing.T) {
	cfg, id := t.TempDir(), "02c5f13e-4af1-44da-a134-90f6fafa5d25"
	to := filepath.Join(cfg, "claude-resume", "archive")
	for _, p := range []string{"projects/p/" + id + ".jsonl", "projects/p/" + id + "/sub", "file-history/" + id + "/f", "session-env/" + id + "/e"} {
		os.MkdirAll(filepath.Join(cfg, filepath.Dir(p)), 0o700)
		os.WriteFile(filepath.Join(cfg, p), nil, 0o600)
	}
	s := &session{path: filepath.Join(cfg, "projects/p", id+".jsonl"), id: id}
	if dsts, err := move(nil, s, to); err != nil || len(dsts) != 4 || len(files(s)) != 4 || root(s) != to {
		t.Fatal(dsts, err, files(s))
	}
	os.MkdirAll(filepath.Join(cfg, "file-history", id), 0o700)
	if _, err := move(nil, s, cfg); !errors.Is(err, fs.ErrExist) || root(s) != to {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(cfg, "file-history", id))
	if _, err := move(nil, s, cfg); err != nil || len(files(s)) != 4 || root(s) != cfg {
		t.Fatal(err, files(s))
	}
}

func TestPurge(t *testing.T) {
	dir := t.TempDir()
	old, recent := &session{path: filepath.Join(dir, "projects/p/a.jsonl"), id: "a", mod: time.Now().AddDate(0, 0, -31)}, &session{path: filepath.Join(dir, "projects/p/b.jsonl"), id: "b", mod: time.Now()}
	os.MkdirAll(filepath.Join(dir, "projects/p"), 0o700)
	os.WriteFile(old.path, nil, 0o600)
	os.WriteFile(recent.path, nil, 0o600)
	if n, err := purge([]*session{old, recent}, time.Now().AddDate(0, 0, -30)); n != 1 || err != nil || len(files(old)) != 0 || len(files(recent)) != 1 {
		t.Fatal(n, err)
	}
}

func TestPin(t *testing.T) {
	config, pinned = t.TempDir(), t.TempDir()
	id := "02c5f13e-4af1-44da-a134-90f6fafa5d25"
	live := parts(config, "p", id)
	write := func(p, text string) {
		os.MkdirAll(filepath.Dir(p), 0o700)
		os.WriteFile(p, []byte(text), 0o600)
	}
	write(live[0], "one\n")
	write(filepath.Join(live[1], "subagents", "a.jsonl"), "sub")
	write(filepath.Join(live[2], "f"), "backup")
	write(filepath.Join(live[3], "e"), "env")
	old := time.Now().AddDate(0, 0, -40)
	os.Chtimes(live[0], old, old)
	s := &session{path: live[0], id: id}
	if msg, ok := pin(s); !ok || !s.pinned {
		t.Fatal(msg)
	}
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }

	f, _ := os.OpenFile(live[0], os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("two\n")
	f.Close()
	if got := read(parts(pinned, "p", id)[0]); got != "one\ntwo\n" {
		t.Fatalf("pinned name does not see appends: %q", got)
	}

	for _, p := range live {
		os.RemoveAll(p)
	}
	if !keep(scan(filepath.Join(config, "projects"))) {
		t.Fatal("keep restored nothing")
	}
	if read(live[0]) != "one\ntwo\n" || read(filepath.Join(live[1], "subagents", "a.jsonl")) != "sub" || read(filepath.Join(live[2], "f")) != "backup" || read(filepath.Join(live[3], "e")) != "env" {
		t.Fatal("restore lost files")
	}
	if st, _ := os.Stat(live[0]); time.Since(st.ModTime()) > time.Minute {
		t.Fatal("restored transcript keeps its old date")
	}

	write(live[0]+".tmp", "compacted\n")
	os.Rename(live[0]+".tmp", live[0])
	all := scan(filepath.Join(config, "projects"))
	keep(all)
	if read(parts(pinned, "p", id)[0]) != "compacted\n" || !all[0].pinned {
		t.Fatal("pinned name not renewed after a rewrite")
	}

	moved := parts(config, "q", id)
	os.MkdirAll(filepath.Dir(moved[0]), 0o700)
	os.Rename(live[0], moved[0])
	os.Rename(live[1], moved[1])
	keep(scan(filepath.Join(config, "projects")))
	if _, err := os.Stat(live[0]); err == nil || read(parts(pinned, "q", id)[0]) != "compacted\n" {
		t.Fatal("relocated session restored into its old folder or not followed")
	}

	os.Chtimes(moved[0], old, old)
	unpin(&session{path: moved[0], id: id, mod: old})
	if files(&session{path: parts(pinned, "q", id)[0], id: id}) != nil || read(moved[0]) != "compacted\n" {
		t.Fatal("unpin left pinned names or changed the session")
	}
	if st, _ := os.Stat(moved[0]); time.Since(st.ModTime()) > time.Minute {
		t.Fatal("unpinning an old session keeps its old date")
	}
}

func TestPreview(t *testing.T) {
	dir := backupTestStores(t)
	id := "02c5f13e-4af1-44da-a134-90f6fafa5d25"
	text := strings.Join([]string{
		`{"type":"user","gitBranch":"feat/limit","message":{"content":"fix the\tlimiter"}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"first reply"}],"usage":{"input_tokens":100,"cache_read_input_tokens":1000}}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"ignored"}]}}`,
		`{"type":"user","message":{"content":"fix the limiter"}}`,
		`{"type":"user","isMeta":true,"message":{"content":"meta"}}`,
		`{"type":"user","message":{"content":"<command-name>/compact</command-name>"}}`,
		`{"type":"user","message":{"content":"add a test"}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"done\nall good"}],"usage":{"input_tokens":2,"cache_creation_input_tokens":1600}}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash"}]}}`,
		`{"type":"assistant","message":{"content":"plain string"}}`,
	}, "\n")
	s := &session{path: filepath.Join(projects, "-dev-app", id+".jsonl"), id: id, cwd: filepath.Join(dir, "dev", "app"), title: "Limiter", mod: time.Date(2026, 10, 22, 11, 47, 0, 0, time.Local)}
	backupTestWrite(t, s.path, text)
	want := []line{
		{"1", "Limiter"},
		{"2", "~" + string(filepath.Separator) + filepath.Join("dev", "app") + "  ·  2026-10-22 11:47  ·  " + id},
		{"2", "feat/limit  ·  " + human(int64(len(text))) + "B  ·  2k tokens"},
		{},
		{"1", "Last reply"},
		{"", "done"},
		{"", "all good"},
		{},
		{"1", "Prompts"},
		{"", "› fix the limiter"},
		{"", "› add a test"},
	}
	if got := preview(s); !reflect.DeepEqual(got, want) {
		t.Fatalf("preview =\n%q\nwant\n%q", got, want)
	}

	empty := &session{path: filepath.Join(projects, "-dev-app", "empty.jsonl"), id: "empty", cwd: "/elsewhere", mod: s.mod}
	text = `{"type":"user","gitBranch":"HEAD","message":{"content":"<ide_opened_file>x</ide_opened_file>"}}`
	backupTestWrite(t, empty.path, text)
	got := preview(empty)
	if got[0].text != "(untitled)" || got[2].text != human(int64(len(text)))+"B" || got[5].text != "(none)" || got[len(got)-1].text != "(none)" {
		t.Fatalf("empty preview = %q", got)
	}
}

func TestColumns(t *testing.T) {
	dir := backupTestStores(t)
	oldSettings, oldColumns, oldLayout := settings, columns, layout
	t.Cleanup(func() { settings, columns, layout = oldSettings, oldColumns, oldLayout })
	settings = filepath.Join(dir, ".claude-resume", "settings.json")
	a := &session{path: filepath.Join(projects, "-w-app", "aaaaaaaa-0000-4000-8000-000000000000.jsonl"), id: "aaaaaaaa-0000-4000-8000-000000000000"}
	b := &session{path: filepath.Join(projects, "-w-longer-folder", "bbbbbbbb-0000-4000-8000-000000000000.jsonl"), id: "bbbbbbbb-0000-4000-8000-000000000000"}
	backupTestWrite(t, a.path, `{"type":"user","cwd":"/w/app","message":{"content":"first"}}`+"\n"+
		`{"type":"assistant","message":{"content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1499}}}`+"\n"+
		`{"type":"assistant","isApiErrorMessage":true,"message":{"content":[{"type":"text","text":"failed"}],"usage":{"input_tokens":0}}}`)
	backupTestWrite(t, b.path, `{"type":"user","cwd":"/w/longer-folder","message":{"content":"second"}}`)
	list := []*session{a, b}

	choose := func(input string) string {
		keys := make(chan []byte, 1)
		out := &exportConfirmOutput{keys: keys, input: input}
		return (&tty{out: bufio.NewWriter(out), keys: keys, wake: make(chan struct{})}).columns(list, false)
	}
	if msg := choose("\x1b[B \x1b[B\x1b[B\x1b[B \x1b[B x\x1b"); msg != "" || !reflect.DeepEqual(columns, oldColumns) {
		t.Fatalf("ESC changed columns: %q %v", msg, columns)
	}
	if _, err := os.Stat(settings); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ESC wrote settings: %v", err)
	}
	if msg := choose("\x1b[B \x1b[B\x1b[B\x1b[B \x1b[B x\r"); msg != "Columns saved." {
		t.Fatalf("msg = %q", msg)
	}
	want := []string{"pin", "title", "folder", "size", "tokens", "id"}
	if !reflect.DeepEqual(columns, want) {
		t.Fatalf("columns = %v, want %v", columns, want)
	}
	columns = nil
	loadSettings()
	if !reflect.DeepEqual(columns, want) {
		t.Fatalf("saved columns = %v, want %v", columns, want)
	}

	list = []*session{{path: a.path, id: a.id, pinned: true}, {path: b.path, id: b.id}}
	loadAll(list)
	got := rows(list, columns)
	wantRows := []string{
		"★  first \t/w/app            292B  1k  aaaaaaaa-0000-4000-8000-000000000000",
		"   second\t/w/longer-folder   71B      bbbbbbbb-0000-4000-8000-000000000000",
	}
	if !reflect.DeepEqual(got, wantRows) {
		t.Fatalf("rows =\n%q\nwant\n%q", got, wantRows)
	}
	if got := rows(list, []string{"pin", "id"}); got[0] != "★  "+a.id {
		t.Fatalf("id only = %q", got)
	}
	if got := rows(list[1:], []string{"pin", "id"}); got[0] != b.id {
		t.Fatalf("pin column without pinned sessions = %q", got)
	}
	if got := rows(list, []string{"date", "title"}); strings.HasSuffix(got[0], " ") || strings.Contains(got[0], "\t") {
		t.Fatalf("left only = %q", got)
	}

	if msg := choose("\x1b[1;2A\x1b[B\x1b[B\x1b[B\x1b[B\x1b[B\x1b[B\x1b[1;2A\x1b[1;2A\x1b[1;2A\x1b[1;2A\x1b[1;2B\x1b[1;2A\r"); msg != "Columns saved." {
		t.Fatalf("msg = %q", msg)
	}
	if want := []string{"pin", "id", "title", "folder", "size", "tokens"}; !reflect.DeepEqual(columns, want) {
		t.Fatalf("moved columns = %v, want %v", columns, want)
	}
	if got := rows(list, columns)[0]; got != "★  "+a.id+"  first \t/w/app            292B  1k" {
		t.Fatalf("moved row = %q", got)
	}
	if got := rows(list, []string{"folder", "id"})[1]; got != "/w/longer-folder  "+b.id {
		t.Fatalf("no title row = %q", got)
	}

	if msg := choose("\x1b[B\x1b[B\x1b[B\x1b[B\x1b[B\x1b[1;2A\x1b[1;2A \r"); msg != "Columns saved." {
		t.Fatalf("msg = %q", msg)
	}
	columns, layout = nil, allColumns
	loadSettings()
	if want := []string{"pin", "id", "title", "folder", "tokens"}; !reflect.DeepEqual(columns, want) {
		t.Fatalf("hidden size columns = %v, want %v", columns, want)
	}
	if choose("\x1b[B\x1b[B\x1b[B \r"); !reflect.DeepEqual(columns, []string{"pin", "id", "size", "title", "folder", "tokens"}) {
		t.Fatalf("size shown again at %v", columns)
	}

	for text, want := range map[string][]string{`{"columns":["id","bogus","date","id"]}`: {"id", "date"}, `{"columns":[]}`: nil, `not json`: {"x"}, `{}`: {"x"}} {
		columns = []string{"x"}
		backupTestWrite(t, settings, text)
		loadSettings()
		if !reflect.DeepEqual(columns, want) {
			t.Fatalf("%s: columns = %v, want %v", text, columns, want)
		}
	}
}

func TestWrap(t *testing.T) {
	for _, c := range []struct {
		text string
		w    int
		want []line
	}{
		{"", 10, []line{{"1", ""}}},
		{"Save as Markdown (CTRL-D)", 12, []line{{"1", "Save as"}, {"cont1", "Markdown"}, {"cont1", "(CTRL-D)"}}},
		{"fits as is", 10, []line{{"1", "fits as is"}}},
		{"abc def", 5, []line{{"1", "abc"}, {"cont1", "def"}}},
		{"abcd efgh", 4, []line{{"1", "abcd"}, {"cont1", "ef"}, {"cont1", "gh"}}},
		{"ab    cd", 4, []line{{"1", "ab"}, {"cont1", "cd"}}},
		{"-d     move it", 8, []line{{"1", "-d"}, {"cont1", "move"}, {"cont1", "it"}}},
		{"a /very/long/path", 8, []line{{"1", "a"}, {"cont1", "/very/"}, {"cont1", "long/p"}, {"cont1", "ath"}}},
		{"  indented", 6, []line{{"1", "  inde"}, {"cont1", "nted"}}},
	} {
		if got := wrap([]line{{"1", c.text}}, c.w); !reflect.DeepEqual(got, c.want) {
			t.Errorf("wrap(%q, %d) = %q, want %q", c.text, c.w, got, c.want)
		}
	}
}
