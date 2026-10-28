#!/usr/bin/env bash
set -euo pipefail

bin_dir=${PREFIX:-$HOME/.local}/bin

say() { printf '==> %s\n' "$*"; }
die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

main() {
  local os arch tmp
  case $(uname -s) in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) die "unsupported system $(uname -s); on Windows, use install.ps1 (see the README)" ;;
  esac
  case $(uname -m) in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) die "unsupported processor $(uname -m)" ;;
  esac

  mkdir -p "$bin_dir"
  tmp="$bin_dir/.claude-resume.tmp"
  curl -fsSL "https://github.com/csmarvz/claude-resume/releases/latest/download/claude-resume-$os-$arch" -o "$tmp" ||
    { rm -f "$tmp"; die "download failed"; }
  chmod 755 "$tmp"
  mv "$tmp" "$bin_dir/claude-resume"
  say "installed $bin_dir/claude-resume"

  case ":$PATH:" in
    *":$bin_dir:"*) ;;
    *) say "$bin_dir is not on your PATH; add this to your shell profile:"
       printf '    export PATH="%s:$PATH"\n' "$bin_dir" ;;
  esac

  command -v claude >/dev/null || say "note: the claude CLI was not found; install Claude Code to resume sessions"
}

main "$@"
