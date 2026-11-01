package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type score struct {
	name      string
	cc        int
	cov, crap float64
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "crap:", err)
		os.Exit(1)
	}
}

func run() error {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	profile := filepath.Join(os.TempDir(), fmt.Sprintf("crap-%d.out", os.Getpid()))
	defer os.Remove(profile)
	test := exec.Command("go", "test", "-coverprofile="+profile, dir)
	test.Stdout, test.Stderr = os.Stderr, os.Stderr
	if err := test.Run(); err != nil {
		return fmt.Errorf("go test failed")
	}
	out, err := exec.Command("go", "tool", "cover", "-func="+profile).Output()
	if err != nil {
		return err
	}
	scores, err := analyze(dir, coverage(string(out)))
	if err != nil {
		return err
	}
	over := 0
	for _, s := range scores {
		if s.crap >= 10 {
			over++
		}
	}
	fmt.Printf("%d functions, %d with CRAP >= 10\n\n%8s %5s %6s  %s\n", len(scores), over, "CRAP", "CC", "COV", "FUNCTION")
	for _, s := range scores {
		fmt.Printf("%8.1f %5d %5.1f%%  %s\n", s.crap, s.cc, s.cov*100, s.name)
	}
	return nil
}

func coverage(out string) map[string]float64 {
	cov := map[string]float64{}
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) != 3 || !strings.HasSuffix(f[2], "%") {
			continue
		}
		loc := strings.TrimSuffix(f[0], ":")
		i := strings.LastIndex(loc, ":")
		if i < 0 {
			continue
		}
		v, _ := strconv.ParseFloat(strings.TrimSuffix(f[2], "%"), 64)
		cov[filepath.Base(loc[:i])+loc[i:]] = v / 100
	}
	return cov
}

func analyze(dir string, cov map[string]float64) ([]score, error) {
	pkg, err := build.Default.ImportDir(dir, 0)
	if err != nil {
		return nil, err
	}
	fs := token.NewFileSet()
	var scores []score
	for _, name := range pkg.GoFiles {
		f, err := parser.ParseFile(fs, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, err
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			cc := complexity(fn.Body)
			key := name + ":" + strconv.Itoa(fs.Position(fn.Pos()).Line)
			c, ok := cov[key]
			if !ok {
				return nil, fmt.Errorf("no coverage for %s %s", key, fn.Name.Name)
			}
			scores = append(scores, score{name + " " + fn.Name.Name, cc, c, float64(cc*cc)*math.Pow(1-c, 3) + float64(cc)})
		}
	}
	sort.SliceStable(scores, func(i, j int) bool { return scores[i].crap > scores[j].crap })
	return scores, nil
}

func complexity(body ast.Node) int {
	cc := 1
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			cc++
		case *ast.CaseClause:
			if n.List != nil {
				cc++
			}
		case *ast.CommClause:
			if n.Comm != nil {
				cc++
			}
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				cc++
			}
		}
		return true
	})
	return cc
}
