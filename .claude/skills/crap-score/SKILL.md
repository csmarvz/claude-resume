---
name: crap-score
description: Use when changing Go code in claude-resume and checking whether a function is too complex for its test coverage, when asked for CRAP scores, or when looking for the riskiest functions to test or split.
---

# CRAP score

`tools/crap` gives each function in the main package a CRAP score:

```text
CRAP = CC² × (1 − coverage)³ + CC
```

`CC` is cyclomatic complexity: 1, plus one for each `if`, `for`, `range`, `case` or `select` clause other than `default`, `&&` and `||`. The rules are the Go rules of [crapper](https://github.com/unclebob/crapper). Coverage is the function's statement coverage from `go test`.

Below 5 is low risk, 5 to 30 is worth a look, and above 30 is complex and under-tested.

## Run

From the repository root:

```bash
go run ./tools/crap
```

It runs `go test -coverprofile` on the main package, so the tests must pass; on a failure it prints the test output and exits 1. The table goes to stdout, worst score first, with the `go test` output on stderr. Only files that build on the current OS are scored: `sys_windows.go` on Windows, `sys_unix.go` elsewhere.

## Using the scores

1. Run it before the change and keep the rows for the functions you will touch.
2. Make the change, run it again, and compare those rows.
3. A function you touched that is at 10 or more: test it, split it, or both. A test calls the function the way its caller does and checks the result the caller sees.
4. Report the before and after scores of the touched functions in the PR description.

There is no CI threshold. The 44 functions at 10 or more on 0.0.13-alpha (measured on Linux) predate the tool, so lower them when you are already changing them rather than in a separate cleanup.

Functions that read the terminal (`pick`, `command`) or the network (`update`, `download`) score high because tests cannot reach them. Lowering those means moving the logic into functions that take plain values and return results, then testing those.
