#!/usr/bin/env bash
set -euo pipefail

repo_raw=https://raw.githubusercontent.com/csmarvz/claude-resume/main
bin_dir=${PREFIX:-$HOME/.local}/bin

say() { printf '==> %s\n' "$*"; }
die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

pkg_install() {
  local sudo=
  if (( EUID != 0 )) && ! command -v brew >/dev/null; then
    command -v sudo >/dev/null || die "need root or sudo to install $*"
    sudo=sudo
  fi
  if command -v brew >/dev/null; then brew install "$@"
  elif command -v apt-get >/dev/null; then $sudo apt-get update -qq && $sudo apt-get install -y "$@"
  elif command -v dnf >/dev/null; then $sudo dnf install -y "$@"
  elif command -v pacman >/dev/null; then $sudo pacman -S --needed --noconfirm "$@"
  elif command -v zypper >/dev/null; then $sudo zypper install -y "$@"
  elif command -v apk >/dev/null; then $sudo apk add "$@"
  else return 1
  fi
}

main() {
  local missing=() dep src tmp
  for dep in jq fzf; do
    command -v "$dep" >/dev/null || missing+=("$dep")
  done

  if (( ${#missing[@]} )); then
    say "installing ${missing[*]}"
    pkg_install "${missing[@]}" || die "could not install ${missing[*]}; install them manually and re-run"
  fi

  mkdir -p "$bin_dir"
  src="$(dirname "${BASH_SOURCE[0]:-}")/claude-resume"
  if [[ -n ${BASH_SOURCE[0]:-} && -f $src ]]; then
    install -m 755 "$src" "$bin_dir/claude-resume"
  else
    tmp="$bin_dir/.claude-resume.tmp"
    curl -fsSL "$repo_raw/claude-resume" -o "$tmp" || { rm -f "$tmp"; die "download failed"; }
    chmod 755 "$tmp"
    mv "$tmp" "$bin_dir/claude-resume"
  fi
  say "installed $bin_dir/claude-resume"

  case ":$PATH:" in
    *":$bin_dir:"*) ;;
    *) say "$bin_dir is not on your PATH; add this to your shell profile:"
       printf '    export PATH="%s:$PATH"\n' "$bin_dir" ;;
  esac

  command -v claude >/dev/null || say "note: the claude CLI was not found; install Claude Code to resume sessions"
}

main "$@"
