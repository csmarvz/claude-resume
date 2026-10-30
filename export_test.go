package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

const exportID = "02c5f13e-4af1-44da-a134-90f6fafa5d25"

func exportMessage(role string, content any) string {
	b, _ := json.Marshal(map[string]any{"type": role, "message": map[string]any{"content": content}})
	return string(b)
}

func TestExportMarkdown(t *testing.T) {
	long := strings.Repeat("日本語", 24000)
	input := strings.Join([]string{
		exportMessage("user", "<real-user-text>\n```go\nx := 1\n```"),
		exportMessage("assistant", []map[string]any{{"type": "thinking", "thinking": "private"}, {"type": "text", "text": long}, {"type": "tool_use", "name": "hidden"}}),
		`{malformed`,
		exportMessage("user", []map[string]any{{"type": "tool_result", "content": "secret"}, {"type": "text", "text": "again"}, {"type": "image", "source": "hidden"}}),
		exportMessage("user", "again"),
		`{"type":"user","isMeta":true,"message":{"content":"internal"}}`,
		`{"type":"user","isCompactSummary":true,"message":{"content":"summary"}}`,
		exportMessage("system", "internal"),
		exportMessage("user", "<local-command-stdout>Compacted</local-command-stdout>"),
		exportMessage("user", "<local-command-caveat>Local command</local-command-caveat>"),
		exportMessage("user", "<command-name>/compact</command-name>"),
		exportMessage("user", "<command-message>unslop:unslop</command-message>\n<command-name>/unslop</command-name>"),
		exportMessage("user", "<task-notification>background work completed</task-notification>"),
		exportMessage("user", "<bash-input>ls</bash-input>"),
		exportMessage("user", "<bash-stdout>files</bash-stdout>"),
		exportMessage("user", "<bash-stderr>failed</bash-stderr>"),
		exportMessage("user", "<session-start-hook>hook output</session-start-hook>"),
		exportMessage("user", "<tick>scheduler output</tick>"),
		exportMessage("user", "<goal>background task</goal>"),
		exportMessage("user", "[Request interrupted by user for tool use]"),
		exportMessage("user", "  <ide_opened_file>file.go</ide_opened_file>\n"),
		exportMessage("user", "<ide_selection>selection</ide_selection>"),
		`{"type":"assistant","isApiErrorMessage":true,"message":{"content":[{"type":"text","text":"API Error: synthetic failure"}]}}`,
		exportMessage("assistant", "API Error: an explanation written by Claude"),
		exportMessage("assistant", "final reply"),
	}, "\n")
	s := &session{path: filepath.Join(t.TempDir(), "session.jsonl")}
	if err := os.WriteFile(s.path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := exportMarkdown(s, &output); err != nil {
		t.Fatal(err)
	}
	want := "## User\n\n<real-user-text>\n```go\nx := 1\n```\n\n## Claude\n\n" + long + "\n\n## User\n\nagain\n\nagain\n\n## Claude\n\nAPI Error: an explanation written by Claude\n\nfinal reply\n\n"
	if output.String() != want {
		t.Fatalf("export changed conversation content: got %d bytes, want %d", output.Len(), len(want))
	}
	if err := exportMarkdown(s, exportFailWriter{}); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("write error lost: %v", err)
	}
}

type exportFailWriter struct{}

func (exportFailWriter) Write([]byte) (int, error) { return 0, fs.ErrPermission }

func TestExportName(t *testing.T) {
	for _, title := range []string{"", "... ", `CON`, `a/b\c:*?"<>|`, strings.Repeat("😀", 200)} {
		s := &session{loaded: true, title: title, id: exportID}
		path := exportName(s)
		base := filepath.Base(path)
		if filepath.Dir(path) != "." || !strings.HasSuffix(base, "--"+exportID+".md") || strings.ContainsAny(base, `<>:"/\|?*`) || len(base) > 255 || !utf8.ValidString(base) {
			t.Errorf("unsafe export filename for %q: %q", title, path)
		}
	}
}

func TestExportPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	s := &session{loaded: true, title: "session title", id: exportID}
	file := filepath.Join(home, "existing.md")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ input, want string }{
		{"~/export.md", filepath.Join(home, "export.md")},
		{`~\export.md`, filepath.Join(home, "export.md")},
		{"~", filepath.Join(home, filepath.Base(exportName(s)))},
		{home, filepath.Join(home, filepath.Base(exportName(s)))},
		{file, file},
	} {
		if got, err := exportPath(s, c.input); err != nil || got != c.want {
			t.Errorf("exportPath(%q) = %q, %v; want %q", c.input, got, err, c.want)
		}
	}
	for _, c := range []struct{ input, message string }{
		{"", "enter an export path"},
		{filepath.Join(home, "nodir", "x.md"), `folder "` + filepath.Join(home, "nodir") + `" does not exist`},
		{filepath.Join(file, "x.md"), "is not a folder"},
	} {
		_, err := exportPath(s, c.input)
		if err == nil || !strings.Contains(err.Error(), c.message) || strings.Contains(err.Error(), ".claude-resume-export-") {
			t.Errorf("unhelpful error for %q: %v", c.input, err)
		}
	}
}

func TestExportSession(t *testing.T) {
	dir := t.TempDir()
	source := []byte(exportMessage("user", "keep me") + "\n{incomplete")
	s := &session{path: filepath.Join(dir, "session.jsonl")}
	if err := os.WriteFile(s.path, source, 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "export.md")
	if err := exportSession(s, path, false); err != nil {
		t.Fatal(err)
	}
	want := []byte("## User\n\nkeep me\n\n")
	if got, _ := os.ReadFile(path); !bytes.Equal(got, want) {
		t.Fatalf("wrong output: %q", got)
	}
	if err := os.WriteFile(path, []byte("old output"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := exportSession(s, path, false); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("unapproved overwrite: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "old output" {
		t.Fatalf("existing output changed: %q", got)
	}
	if err := exportSession(s, path, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, want) {
		t.Fatalf("overwrite did not save Markdown: %q", got)
	}
	if st, err := os.Stat(path); err != nil || (runtime.GOOS != "windows" && st.Mode().Perm() != 0o600) {
		t.Fatalf("export must retain private file permissions: %v, %v", st, err)
	}
	for _, dest := range []string{s.path, dir, filepath.Join(dir, "missing", "export.md")} {
		if err := exportSession(s, dest, true); err == nil {
			t.Errorf("accepted invalid destination %q", dest)
		} else if strings.Contains(err.Error(), ".claude-resume-export-") {
			t.Errorf("temporary path leaked into save error: %v", err)
		}
	}
	alias := filepath.Join(dir, "alias.md")
	if err := os.Link(s.path, alias); err == nil {
		if err := exportSession(s, alias, true); err == nil {
			t.Fatal("overwrote transcript hard link")
		}
	}
	if got, _ := os.ReadFile(s.path); !bytes.Equal(got, source) {
		t.Fatal("source transcript changed")
	}
	if err := exportSession(&session{path: filepath.Join(dir, "absent.jsonl")}, path, true); err == nil {
		t.Fatal("missing source was accepted")
	}
	if err := exportSession(&session{path: dir}, path, true); err == nil {
		t.Fatal("source read error was ignored")
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, want) {
		t.Fatal("failed export destroyed existing output")
	}
	if files, _ := filepath.Glob(filepath.Join(dir, ".claude-resume-export-*")); len(files) != 0 {
		t.Fatalf("temporary files left behind: %v", files)
	}
}

func TestEachLineOpenHandle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript")
	if err := os.WriteFile(path, []byte("first\nsecond\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var lines []string
	err := eachLine(path, func(int, []byte) bool { return true }, func(n int, b []byte) bool {
		lines = append(lines, string(b))
		if n == 0 {
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.WriteString("appended\n"); err != nil {
				t.Fatal(err)
			}
			f.Close()
			if err := os.WriteFile(path+".new", []byte("replacement\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(path+".new", path); err != nil {
				t.Logf("platform refuses replacement while the source handle is open: %v", err)
			}
		}
		return true
	})
	if err != nil || strings.Join(lines, "") != "first\nsecond\nappended\n" {
		t.Fatalf("single-handle read changed: %q, %v", lines, err)
	}
}

func TestExportCLIProcess(t *testing.T) {
	if os.Getenv("CLAUDE_RESUME_EXPORT_TEST") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	os.Exit(2)
}

func TestExportCLI(t *testing.T) {
	for _, pool := range []string{"live", "archive", "trash"} {
		t.Run(pool, func(t *testing.T) {
			dir := t.TempDir()
			cfg := filepath.Join(dir, "config")
			root := cfg
			if pool != "live" {
				root = filepath.Join(dir, ".claude-resume", pool)
			}
			source := filepath.Join(root, "projects", "project", exportID+".jsonl")
			if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
				t.Fatal(err)
			}
			content := exportMessage("user", "hello") + "\n" + exportMessage("assistant", "world")
			if err := os.WriteFile(source, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			run := func(args ...string) (string, error) {
				cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestExportCLIProcess$", "--"}, args...)...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "CLAUDE_RESUME_EXPORT_TEST=1", "CLAUDE_CONFIG_DIR="+cfg, "USERPROFILE="+dir, "HOME="+dir)
				out, err := cmd.CombinedOutput()
				return string(out), err
			}
			out, err := run("--export", exportID[:8])
			if err != nil || !strings.Contains(out, "Saved ") {
				t.Fatalf("default export: %s, %v", out, err)
			}
			def := filepath.Join(dir, "hello--"+exportID+".md")
			if b, err := os.ReadFile(def); err != nil || !strings.Contains(string(b), "## Claude\n\nworld") {
				t.Fatalf("default destination: %q, %v", b, err)
			}
			path := filepath.Join(dir, "path with spaces.md")
			if out, err := run("--export", exportID, path); err != nil {
				t.Fatalf("explicit export: %s, %v", out, err)
			}
			if out, err := run("--export", exportID, path); err == nil || !strings.Contains(out, "needs a terminal") {
				t.Fatalf("unattended overwrite: %s, %v", out, err)
			}
			folder := filepath.Join(dir, "output folder")
			if err := os.Mkdir(folder, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, destination := range []string{folder, "~/home-export.md"} {
				if out, err := run("--export", exportID, destination); err != nil {
					t.Fatalf("friendly destination %q: %s, %v", destination, out, err)
				}
			}
			for _, saved := range []string{filepath.Join(folder, filepath.Base(def)), filepath.Join(dir, "home-export.md")} {
				if b, err := os.ReadFile(saved); err != nil || !strings.Contains(string(b), "## Claude\n\nworld") {
					t.Fatalf("resolved destination %q: %q, %v", saved, b, err)
				}
			}
			if out, err := run("--export", exportID, filepath.Join(dir, "nodir", "x.md")); err == nil || !strings.Contains(out, "does not exist") || strings.Contains(out, ".claude-resume-export-") {
				t.Fatalf("missing-folder error: %s, %v", out, err)
			}
			if out, err := run("--export", "abcdef"); err == nil || !strings.Contains(out, "no session") {
				t.Fatalf("missing ID: %s, %v", out, err)
			}
			duplicate := filepath.Join(root, "projects", "other", exportID+".jsonl")
			if err := os.MkdirAll(filepath.Dir(duplicate), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(duplicate, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := run("--export", exportID); err == nil || !strings.Contains(out, "pick one") {
				t.Fatalf("ambiguous ID: %s, %v", out, err)
			}
			if b, err := os.ReadFile(source); err != nil || string(b) != content {
				t.Fatalf("CLI changed source: %q, %v", b, err)
			}
		})
	}
}

type exportConfirmOutput struct {
	bytes.Buffer
	keys  chan []byte
	input string
}

func (w *exportConfirmOutput) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if w.input != "" {
		w.keys <- []byte(w.input)
		w.input = ""
	}
	return n, err
}

func TestExportSharedConfirmation(t *testing.T) {
	for _, c := range []struct {
		input string
		want  bool
	}{{"Y\r", false}, {"n\x1b[C\r", true}, {"x\x1b[C\r", true}, {"\x1b", false}, {"q", false}, {"\r", false}, {"\x1b[C\r", true}} {
		for _, title := range []string{"Delete session", "Export session"} {
			keys := make(chan []byte, 1)
			out := &exportConfirmOutput{keys: keys, input: c.input}
			terminal := &tty{out: bufio.NewWriter(out), keys: keys, wake: make(chan struct{})}
			var got bool
			if title == "Delete session" {
				got = terminal.confirm("Y confirm · any other key cancels", []line{{"", "session.jsonl"}})
				if strings.Contains(out.String(), "Y confirm") || strings.Contains(out.String(), "any other key cancels") {
					t.Fatal("delete dialog displayed the picker-only prompt")
				}
			} else {
				got = terminal.confirmDialog(title, []line{{"1", "Overwrite this export?"}, {}, {"", "export.md"}})
				if !strings.Contains(out.String(), "Overwrite this export?") || strings.Contains(out.String(), "Delete session") {
					t.Fatal("export dialog showed the wrong action")
				}
			}
			if got != c.want {
				t.Errorf("%s confirmation %q = %v, want %v", title, c.input, got, c.want)
			}
			if !strings.Contains(out.String(), title) || strings.Contains(out.String(), "Y/N") {
				t.Fatal("dialog title or controls changed")
			}
		}
	}
}
