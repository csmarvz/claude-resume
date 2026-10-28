package main

import (
	"sort"
	"strings"
	"unicode"
)

// match reports whether every term fuzzy-matches s, and which runes to highlight.
// A term is case-sensitive only when it contains an uppercase letter.
func match(s string, terms []string) ([]int, bool) {
	text := []rune(s)
	lower := make([]rune, len(text))
	for i, r := range text {
		lower[i] = unicode.ToLower(r)
	}
	var pos []int
	for _, t := range terms {
		in := lower
		if strings.IndexFunc(t, unicode.IsUpper) >= 0 {
			in = text
		}
		p := fuzzy(in, []rune(t))
		if p == nil {
			return nil, false
		}
		pos = append(pos, p...)
	}
	sort.Ints(pos)
	return pos, true
}

// fuzzy finds pat as a subsequence of text, preferring the shortest span that ends at the first full match.
func fuzzy(text, pat []rune) []int {
	j, end := 0, -1
	for i, r := range text {
		if r == pat[j] {
			if j++; j == len(pat) {
				end = i
				break
			}
		}
	}
	if end < 0 {
		return nil
	}
	start := end
	for i, j := end, len(pat)-1; i >= 0; i-- {
		if text[i] == pat[j] {
			if j--; j < 0 {
				start = i
				break
			}
		}
	}
	pos := make([]int, 0, len(pat))
	for i, j := start, 0; i <= end && j < len(pat); i++ {
		if text[i] == pat[j] {
			pos = append(pos, i)
			j++
		}
	}
	return pos
}
