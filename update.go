package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
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
	var out, stderr bytes.Buffer
	cmd := exec.Command("curl", append([]string{"-fsS", "--connect-timeout", "10"}, args...)...)
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	return out.Bytes(), explain(err, stderr.String())
}

func explain(err error, stderr string) error {
	var e *exec.ExitError
	if errors.Is(err, exec.ErrNotFound) {
		return errors.New("curl is needed for updates but isn't installed")
	}
	if !errors.As(err, &e) {
		return err
	}
	switch e.ExitCode() {
	case 5, 6:
		return errors.New("can't find github.com; check your internet connection")
	case 7:
		return errors.New("can't connect to github.com; check your internet connection, proxy or firewall")
	case 28:
		return errors.New("the connection is too slow or stopped answering")
	case 18, 52, 55, 56:
		return errors.New("the connection broke before the download finished")
	case 35, 51, 58, 60, 77:
		return errors.New("the secure connection to github.com failed; check your system clock and any proxy")
	case 22:
		if i := strings.LastIndex(stderr, "error: "); i >= 0 {
			return fmt.Errorf("GitHub answered with error %s", strings.TrimSpace(stderr[i+7:]))
		}
		return errors.New("GitHub answered with an error")
	case 23:
		return errors.New("the download could not be written to disk")
	}
	if msg := strings.TrimSpace(stderr); msg != "" {
		return errors.New(strings.TrimPrefix(msg, "curl: "))
	}
	return err
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
	from, open, tmp := currentVersion(), false, ""
	home, _ = os.UserHomeDir()
	step := func(text string) { fmt.Print(text + "... "); open = true }
	end := func(text string) { fmt.Println(text); open = false }
	fail := func(format string, a ...any) {
		if open {
			fmt.Println()
		}
		if tmp != "" {
			os.Remove(tmp)
		}
		fmt.Fprintf(os.Stderr, "Update failed: "+format+"\nNothing was changed; claude-resume %s is still installed.\n", append(a, from)...)
		os.Exit(1)
	}

	fmt.Printf("claude-resume %s is installed.\n", from)
	step("Checking for a newer release")
	tag, err := latest()
	if err != nil {
		fail("%v.", err)
	}
	remember(tag)
	v := strings.TrimPrefix(tag, "v")
	if !newer(v, from) {
		end("none, " + from + " is the latest.")
		return
	}
	end("found " + v + ".")

	name := "claude-resume-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	base := repo + "/releases/download/" + tag + "/"
	sums, err := get(base + "checksums.txt")
	if err != nil {
		fail("could not download the checksums of %s: %v.", v, err)
	}
	want := ""
	for _, l := range strings.Split(string(sums), "\n") {
		if f := strings.Fields(l); len(f) == 2 && f[1] == name {
			want = f[0]
		}
	}
	if want == "" {
		fail("release %s has no build for this system (%s).", v, name)
	}

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		fail("cannot find the installed program: %v.", err)
	}
	f, err := os.Create(exe + ".new")
	if errors.Is(err, os.ErrPermission) {
		fail("no permission to write to %s. Run the update as the user who installed claude-resume, or install it again with the install command.", filepath.Dir(exe))
	}
	if err != nil {
		fail("cannot write next to %s: %v.", exe, err)
	}
	f.Close()
	tmp = exe + ".new"

	if err := download(base+name, tmp, name); err != nil {
		fail("%v. Try again later.", err)
	}

	step("Checking the download against the release checksum")
	h := sha256.New()
	if f, err = os.Open(tmp); err == nil {
		_, err = io.Copy(h, f)
		f.Close()
	}
	if err != nil {
		fail("cannot read the download: %v.", err)
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		fail("the download is damaged: its checksum doesn't match the release. Try again.")
	}
	end("ok.")

	step("Installing into " + tilde(filepath.Dir(exe)))
	if err := os.Chmod(tmp, 0o755); err != nil {
		fail("cannot make the download executable: %v.", err)
	}
	if err := replace(tmp, exe); err != nil {
		if runtime.GOOS == "windows" {
			fail("cannot replace %s: %v. Close other windows running claude-resume and try again.", exe, err)
		}
		fail("cannot replace %s: %v.", exe, err)
	}
	end("done.")
	fmt.Printf("Updated claude-resume %s to %s.\n", from, v)
}

func amount(n int64) string {
	h := human(n)
	if u := h[len(h)-1]; u >= 'A' && u <= 'Z' {
		return h[:len(h)-1] + " " + string(u) + "B"
	}
	return h + " bytes"
}

// download runs curl into dest and redraws a progress line while it runs.
func download(url, dest, name string) error {
	total := int64(0)
	if out, err := curl("-sIL", "--max-time", "15", url); err == nil {
		for _, l := range strings.Split(string(out), "\n") {
			if k, val, ok := strings.Cut(l, ":"); ok && strings.EqualFold(k, "content-length") {
				total, _ = strconv.ParseInt(strings.TrimSpace(val), 10, 64)
			}
		}
	}
	var stderr bytes.Buffer
	cmd := exec.Command("curl", "-fsSL", "--connect-timeout", "10", "--speed-limit", "1024", "--speed-time", "30", "-o", dest, url)
	cmd.Stderr = &stderr
	tty := term.IsTerminal(int(os.Stdout.Fd()))
	show := func(final bool) {
		n := int64(0)
		if st, err := os.Stat(dest); err == nil {
			n = st.Size()
		}
		line := fmt.Sprintf("Downloading %s... %s", name, amount(n))
		if total > 0 {
			line = fmt.Sprintf("Downloading %s... %s of %s, %d%%", name, amount(n), amount(total), n*100/total)
		}
		if final {
			line = fmt.Sprintf("Downloading %s... %s, done.", name, amount(n))
		}
		fmt.Printf("\r\x1b[K%s", line)
	}
	if !tty {
		fmt.Printf("Downloading %s... ", name)
		show = func(final bool) {
			if final {
				fmt.Print("done.")
			}
		}
	}
	if err := cmd.Start(); err != nil {
		return explain(err, "")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			if err != nil {
				fmt.Println()
				if e := (*exec.ExitError)(nil); errors.As(err, &e) && e.ExitCode() == 28 {
					return errors.New("the download stalled: under 1 KB/s for 30 seconds")
				}
				return explain(err, stderr.String())
			}
			show(true)
			fmt.Println()
			return nil
		case <-tick.C:
			show(false)
		}
	}
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
