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
