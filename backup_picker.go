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
				return 1
			}
			return -1
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

func backupPreview(file string) ([]line, error) {
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
	for _, s := range m.Sessions {
		out = append(out, line{"", printable(s.Category + " · " + s.Project + " · " + s.ID)})
	}
	return out, nil
}

func (t *tty) restoreBackupDialog() string {
	v := &view{edit: true, foot: "Type path · ↑↓ select · ENTER open or restore · ESC cancel"}
	var dir, loadedDir, prefix string
	var entries, matches []os.FileInfo
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
		v.header = []string{"Restore backup · " + printable(dir)}
		v.items = v.items[:0]
		matches = matches[:0]
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
		for _, entry := range entries {
			base := entry.Name()
			if strings.HasPrefix(base, ".") && !strings.HasPrefix(name, ".") {
				continue
			}
			match := strings.HasPrefix(base, name)
			if runtime.GOOS == "windows" {
				match = strings.HasPrefix(strings.ToLower(base), strings.ToLower(name))
			}
			if !match {
				continue
			}
			if base == name {
				v.sel = len(matches)
			}
			matches = append(matches, entry)
			label := printable(base)
			if entry.IsDir() {
				label += string(os.PathSeparator)
			} else {
				label += "\t" + entry.ModTime().Format("2006-01-02 15:04") + " · " + human(entry.Size()) + "B"
			}
			v.items = append(v.items, label)
		}
	}
	v.preview = func(i int) []line {
		if i < 0 || matches[i].IsDir() {
			return nil
		}
		file := filepath.Join(dir, matches[i].Name())
		if file != previewFile {
			previewFile = file
			var err error
			previewLines, err = backupPreview(file)
			if err != nil {
				previewLines = []line{{red, printable(err.Error())}}
			}
		}
		return previewLines
	}
	v.complete()
	for {
		key, i := t.run(v)
		if key != "enter" {
			return "Cancelled."
		}
		input := string(v.query)
		if i >= 0 {
			input = prefix + matches[i].Name()
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
			case info.IsDir():
				if !os.IsPathSeparator(input[len(input)-1]) {
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
		msg, err := backupRestore(file, func(lines []line) string { return backupChoose(t, lines) })
		if err != nil {
			return printable(msg + " " + err.Error())
		}
		return printable(msg)
	}
}
