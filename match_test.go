package main

import (
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
