#!/usr/bin/env bash
set -euo pipefail

bin_dir=${PREFIX:-$HOME/.local}/bin

say() { printf '==> %s\n' "$*"; }
die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

main() {
  local os arch tmp name want got
  local base=https://github.com/csmarvz/claude-resume/releases/latest/download
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
  name=claude-resume-$os-$arch
  curl -fsSL "$base/$name" -o "$tmp" || { rm -f "$tmp"; die "download failed"; }
  want=$(curl -fsSL "$base/checksums.txt" | awk -v n="$name" '$2 == n { print $1 }') || true
  got=$({ sha256sum "$tmp" 2>/dev/null || shasum -a 256 "$tmp"; } | cut -d ' ' -f 1)
  [ -n "$want" ] && [ "$got" = "$want" ] || { rm -f "$tmp"; die "the download does not match its checksum; nothing was installed"; }
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
