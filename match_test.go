package main

import (
	"reflect"
	"strings"
	"testing"
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
	} {
		if got := width(c.s); got != c.w {
			t.Errorf("width(%q) = %d, want %d", c.s, got, c.w)
		}
		if got := cut(c.s, 4); got != c.cut4 {
			t.Errorf("cut(%q, 4) = %q, want %q", c.s, got, c.cut4)
		}
	}
}
