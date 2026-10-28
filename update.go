package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const repo = "https://github.com/csmarvz/claude-resume"

var version = "dev"

func currentVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return strings.TrimPrefix(bi.Main.Version, "v")
	}
	return version
}

func curl(args ...string) ([]byte, error) {
	out, err := exec.Command("curl", append([]string{"-fsS", "--connect-timeout", "10"}, args...)...).Output()
	if e, ok := err.(*exec.ExitError); ok && len(e.Stderr) > 0 {
		err = fmt.Errorf("%s", strings.TrimSpace(string(e.Stderr)))
	}
	return out, err
}

func label() string {
	if v := currentVersion(); v != "dev" {
		return "v" + v
	}
	return "dev"
}

// latest reads the newest release tag from the redirect of /releases/latest, which needs no API token.
func latest() (string, error) {
	out, err := curl("-I", "--max-time", "15", "-o", os.DevNull, "-w", "%{redirect_url}", repo+"/releases/latest")
	if err != nil {
		return "", err
	}
	loc := string(out)
	i := strings.LastIndex(loc, "/tag/")
	if i < 0 {
		return "", fmt.Errorf("unexpected answer from GitHub")
	}
	return loc[i+5:], nil
}

func get(url string) ([]byte, error) { return curl("-L", "--max-time", "60", url) }

func cacheFile() string {
	d, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "claude-resume", "latest-version")
}

func remember(tag string) {
	if f := cacheFile(); f != "" {
		os.MkdirAll(filepath.Dir(f), 0o755)
		os.WriteFile(f, []byte(tag+"\n"), 0o644)
	}
}

// notice returns the footer text for a newer release found by an earlier check,
// and refreshes that check in the background when it is a day old.
func notice() string {
	f := cacheFile()
	if os.Getenv("CLAUDE_RESUME_NO_UPDATE_CHECK") != "" || currentVersion() == "dev" || f == "" {
		return ""
	}
	if st, err := os.Stat(f); err != nil || time.Since(st.ModTime()) > 24*time.Hour {
		go func() {
			if tag, err := latest(); err == nil {
				remember(tag)
			}
		}()
	}
	b, _ := os.ReadFile(f)
	v := strings.TrimPrefix(strings.TrimSpace(string(b)), "v")
	if v == "" || !newer(v, currentVersion()) {
		return ""
	}
	return label() + " → v" + v + ": run claude-resume update"
}

func update() {
	tag, err := latest()
	if err != nil {
		die("could not check for updates: %v", err)
	}
	remember(tag)
	v := strings.TrimPrefix(tag, "v")
	if !newer(v, currentVersion()) {
		fmt.Printf("claude-resume %s is up to date\n", currentVersion())
		return
	}
	name := "claude-resume-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	base := repo + "/releases/download/" + tag + "/"
	sums, err := get(base + "checksums.txt")
	if err != nil {
		die("could not download the checksums of %s: %v", tag, err)
	}
	want := ""
	for _, l := range strings.Split(string(sums), "\n") {
		if f := strings.Fields(l); len(f) == 2 && f[1] == name {
			want = f[0]
		}
	}
	if want == "" {
		die("release %s has no build for %s", tag, name)
	}
	bin, err := get(base + name)
	if err != nil {
		die("download failed: %v", err)
	}
	if sum := sha256.Sum256(bin); hex.EncodeToString(sum[:]) != want {
		die("the download of %s does not match its checksum; nothing was changed", name)
	}
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		die("cannot find the installed program: %v", err)
	}
	tmp := exe + ".new"
	if err := os.WriteFile(tmp, bin, 0o755); err != nil {
		die("cannot write next to %s: %v", exe, err)
	}
	if err := replace(tmp, exe); err != nil {
		os.Remove(tmp)
		die("cannot replace %s: %v", exe, err)
	}
	fmt.Printf("updated claude-resume %s to %s\n", currentVersion(), v)
}

// newer reports whether version a is later than b, comparing major.minor.patch
// numerically and treating a release as later than its pre-releases.
func newer(a, b string) bool {
	an, ap, _ := strings.Cut(a, "-")
	bn, bp, _ := strings.Cut(b, "-")
	as, bs := strings.Split(an, "."), strings.Split(bn, ".")
	for i := range max(len(as), len(bs)) {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			return x > y
		}
	}
	if ap == "" || bp == "" {
		return ap == "" && bp != ""
	}
	return ap > bp
}
