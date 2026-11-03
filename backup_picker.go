package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

func backupInputPath(input string) (string, error) {
	if input == "~" || strings.HasPrefix(input, "~/") || (runtime.GOOS == "windows" && strings.HasPrefix(input, `~\`)) {
		dir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		input = filepath.Join(dir, input[1:])
	}
	return filepath.Abs(input)
}

func backupList(dir string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var matches []os.FileInfo
	for _, entry := range entries {
		link := entry.Type()&(os.ModeSymlink|os.ModeIrregular) != 0
		if !entry.IsDir() && !link && !strings.EqualFold(filepath.Ext(entry.Name()), ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if link {
			info, err = os.Stat(filepath.Join(dir, entry.Name()))
			if err != nil || !info.IsDir() {
				continue
			}
		}
		if info.IsDir() || info.Mode().IsRegular() {
			matches = append(matches, info)
		}
	}
	slices.SortStableFunc(matches, func(a, b os.FileInfo) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}
			return 1
		}
		if !a.IsDir() {
			if order := b.ModTime().Compare(a.ModTime()); order != 0 {
				return order
			}
		}
		return strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
	})
	return matches, nil
}

func backupMatches(entries []os.FileInfo, prefix string) []os.FileInfo {
	folded := strings.ToLower(prefix)
	var matches []os.FileInfo
	for _, entry := range entries {
		name := entry.Name()
		if (!strings.HasPrefix(name, ".") || strings.HasPrefix(prefix, ".")) && strings.HasPrefix(strings.ToLower(name), folded) {
			matches = append(matches, entry)
		}
	}
	return matches
}

func backupPreview(file string, local map[[3]string]*session) ([]line, error) {
	info, err := os.Lstat(file)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", file)
	}
	z, err := zip.OpenReader(file)
	if z != nil {
		defer z.Close()
	}
	if err != nil {
		return nil, err
	}
	var manifest *zip.File
	for _, entry := range z.File {
		if entry.Name == "manifest.json" {
			manifest = entry
			break
		}
	}
	m, err := backupReadManifest(manifest)
	if err != nil {
		return nil, err
	}
	out := []line{{"1", fmt.Sprintf("Sessions in backup: %d", len(m.Sessions))}}
	stores := backupStores()
	for _, s := range m.Sessions {
		title := s.ID
		if existing := local[[3]string{stores[s.Category], s.Project, s.ID}]; existing != nil {
			title = name(existing)
		}
		out = append(out, line{"", printable(s.Modified.Local().Format("2006-01-02 15:04") + " · " + title + " · " + s.Category)})
	}
	return out, nil
}

func (t *tty) restoreBackupDialog(sessions []*session) string {
	v := &view{edit: true, foot: "↑↓ select · TAB complete · ENTER open/restore · ESC cancel"}
	local := make(map[[3]string]*session)
	for _, s := range sessions {
		local[[3]string{root(s), folder(s), s.id}] = s
	}
	var dir, loadedDir, prefix string
	var entries, matches []os.FileInfo
	var parent bool
	var previewFile string
	var previewLines []line
	v.complete = func() {
		var name string
		prefix, name = filepath.Split(string(v.query))
		if filepath.IsAbs(prefix) && !os.IsPathSeparator(prefix[len(prefix)-1]) {
			prefix += string(os.PathSeparator)
		}
		var err error
		dir, err = backupInputPath(prefix)
		v.header = []string{"Restore backup · " + strings.ReplaceAll(printable(homePath(dir)), "\n", " ")}
		v.items = v.items[:0]
		matches, parent = nil, false
		if err == nil && loadedDir != dir {
			entries, err = backupList(dir)
			loadedDir = ""
			if err == nil {
				loadedDir = dir
			}
		}
		if err != nil {
			v.header = append(v.header, printable(err.Error()))
			return
		}
		matches = backupMatches(entries, name)
		parent = name == "" && filepath.Dir(dir) != dir
		offset := 0
		if parent {
			v.items = append(v.items, "..")
			offset = 1
			if len(matches) > 0 {
				v.sel = 1
			}
		}
		newest := -1
		for i, entry := range matches {
			base := entry.Name()
			if base == name {
				v.sel = i + offset
			}
			label := printable(base)
			if entry.IsDir() {
				label += string(os.PathSeparator)
			} else {
				if newest < 0 {
					newest = i + offset
				}
				label += "\t" + entry.ModTime().Format("2006-01-02 15:04") + " · " + human(entry.Size()) + "B"
			}
			v.items = append(v.items, label)
		}
		if name == "" && newest >= 0 {
			v.sel = newest
		}
	}
	v.preview = func(i int) []line {
		if parent {
			i--
		}
		if i < 0 || matches[i].IsDir() {
			return nil
		}
		file := filepath.Join(dir, matches[i].Name())
		if file != previewFile {
			previewFile = file
			var err error
			previewLines, err = backupPreview(file, local)
			if err != nil {
				previewLines = []line{{red, printable(err.Error())}}
			}
		}
		return previewLines
	}
	v.complete()
	for {
		key, i := t.run(v)
		if key != "enter" && key != "tab" {
			return "Cancelled."
		}
		input := string(v.query)
		if i >= 0 {
			if parent && i == 0 {
				input = prefix + ".."
			} else {
				if parent {
					i--
				}
				input = prefix + matches[i].Name()
			}
		}
		if input == "" {
			v.header = append(v.header[:1], "Enter a ZIP path.")
			continue
		}
		file, err := backupInputPath(input)
		if err == nil {
			var info os.FileInfo
			info, err = os.Stat(file)
			switch {
			case err != nil:
			case info.IsDir() || key == "tab" && info.Mode().IsRegular():
				if info.IsDir() && !os.IsPathSeparator(input[len(input)-1]) {
					input += string(os.PathSeparator)
				}
				v.query = []rune(input)
				v.cur = len(v.query)
				v.dim, v.sel, v.top, v.pscroll = false, 0, 0, 0
				v.complete()
				continue
			case !info.Mode().IsRegular():
				err = fmt.Errorf("not a regular file: %s", file)
			}
		}
		if err != nil {
			v.header = append(v.header[:1], printable(err.Error()))
			continue
		}
		msg, err := backupRestore(file, func(lines []line) string { return backupChoose(t, lines) }, t.backupProgress(v))
		if err != nil {
			return printable(msg + " " + err.Error())
		}
		return printable(msg)
	}
}
