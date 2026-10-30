package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Claude's generated user messages can lack isMeta; keep ordinary XML-like prompts.
var exportInternal = regexp.MustCompile(`^(?:<local-command-(?:stdout|stderr|caveat)>|<command-name>|<session-start-hook>|<tick>|<goal>|\[Request interrupted by user[^\]]*\]|\s*<ide_opened_file>[\s\S]*</ide_opened_file>\s*$|\s*<ide_selection>[\s\S]*</ide_selection>\s*$)`)

func exportName(s *session) string {
	if !s.loaded {
		s.load()
	}
	var title strings.Builder
	for _, r := range s.title {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			r = '-'
		}
		if title.Len()+utf8.RuneLen(r) > 100 {
			break
		}
		title.WriteRune(r)
	}
	base := strings.Trim(title.String(), " .")
	if base == "" {
		base = "session"
	}
	return "." + string(filepath.Separator) + base + "--" + s.id + ".md"
}

func exportText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct{ Type, Text string }
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var texts []string
	for _, b := range blocks {
		if b.Type == "text" {
			texts = append(texts, b.Text)
		}
	}
	return strings.Join(texts, "\n")
}

func exportMarkdown(s *session, w io.Writer) error {
	var writeErr error
	err := eachLine(s.path, func(int, []byte) bool { return true }, func(_ int, b []byte) bool {
		var r record
		if json.Unmarshal(b, &r) != nil || r.IsMeta || r.IsCompactSummary || r.IsApiErrorMessage || (r.Type != "user" && r.Type != "assistant") {
			return true
		}
		text := exportText(r.Message.Content)
		if text == "" || (r.Type == "user" && exportInternal.MatchString(text)) {
			return true
		}
		role := "User"
		if r.Type == "assistant" {
			role = "Claude"
		}
		_, writeErr = fmt.Fprintf(w, "## %s\n\n%s\n\n", role, text)
		return writeErr == nil
	})
	if err != nil {
		return err
	}
	return writeErr
}

func exportSession(s *session, path string, overwrite bool) error {
	source, err := os.Stat(s.path)
	if err != nil {
		return err
	}
	if dst, err := os.Lstat(path); err == nil {
		if !dst.Mode().IsRegular() || os.SameFile(source, dst) {
			return errors.New("choose a regular destination file different from the transcript")
		}
		if !overwrite {
			return fs.ErrExist
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".claude-resume-export-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	w := bufio.NewWriter(f)
	err = exportMarkdown(s, w)
	if err == nil {
		err = w.Flush()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if !overwrite {
		// Reserve a new name exclusively so a file created during rendering needs approval too.
		reserved, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		if err := reserved.Close(); err != nil {
			os.Remove(path)
			return err
		}
	}
	if err := os.Rename(f.Name(), path); err != nil {
		if !overwrite {
			os.Remove(path)
		}
		return err
	}
	return nil
}
