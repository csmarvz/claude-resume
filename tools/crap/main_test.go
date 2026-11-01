package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestScores(t *testing.T) {
	dir := t.TempDir()
	src := `package x

func plain() {}

// busy has a doc comment.
func busy(a, b bool, xs []int, c chan int) {
	if a && b || a {
	}
	for _, x := range xs {
		switch x {
		case 1, 2:
		case 3:
		default:
		}
	}
	for {
		select {
		case <-c:
		default:
		}
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	scores, err := analyze(dir, coverage("example.com/x/x.go:6:\tbusy\t50.0%\nexample.com/x/x.go:3:\tplain\t100.0%\ntotal:\t(statements)\t75.0%\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 2 || scores[0].name != "x.go busy" || scores[0].cc != 9 || scores[0].cov != 0.5 {
		t.Fatalf("got %+v", scores)
	}
	if want := 81*math.Pow(0.5, 3) + 9; scores[0].crap != want {
		t.Fatalf("crap = %v, want %v", scores[0].crap, want)
	}
	if s := scores[1]; s.cc != 1 || s.crap != 1 {
		t.Fatalf("plain = %+v", s)
	}
}
