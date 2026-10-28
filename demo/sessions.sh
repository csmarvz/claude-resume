#!/usr/bin/env bash
set -euo pipefail

out=${1:?usage: sessions.sh <dir>}/projects
home=/home/demo

session() {
  local cwd=$1 title=$2 age=$3 reply=$4 dir h id f p parent=null uuid text n=0
  shift 4
  h=$(printf '%s' "$title" | md5sum)
  id=${h:0:8}-${h:8:4}-4${h:13:3}-a${h:17:3}-${h:20:12}
  dir=$out/$(tr '/' '-' <<<"$cwd")
  f=$dir/$id.jsonl
  mkdir -p "$dir"
  : >"$f"
  for p in "$@"; do
    n=$((n + 1))
    text="On it."
    [ $n = $# ] && text=$reply
    uuid=$(cat /proc/sys/kernel/random/uuid)
    jq -nc --arg cwd "$cwd" --arg id "$id" --arg u "$uuid" --argjson parent "$parent" --arg t "$age" --arg p "$p" \
      '{type: "user", parentUuid: $parent, uuid: $u, sessionId: $id, timestamp: ($t | sub(" "; "T") + ":00.000Z"), cwd: $cwd, gitBranch: "main", isSidechain: false, userType: "external", version: "2.1.312",
        message: {role: "user", content: $p}}' >>"$f"
    parent=\"$uuid\"
    uuid=$(cat /proc/sys/kernel/random/uuid)
    jq -nc --arg cwd "$cwd" --arg id "$id" --arg u "$uuid" --argjson parent "$parent" --arg t "$age" --arg r "$text" \
      '{type: "assistant", parentUuid: $parent, uuid: $u, sessionId: $id, timestamp: ($t | sub(" "; "T") + ":30.000Z"), cwd: $cwd, gitBranch: "main", isSidechain: false, userType: "external", version: "2.1.312",
        message: {id: ("msg_" + $u), type: "message", role: "assistant", model: "claude-opus-5-5", content: [{type: "text", text: $r}], stop_reason: "end_turn", stop_sequence: null,
          usage: {input_tokens: ($r | length * 97), output_tokens: 120}}}' >>"$f"
    parent=\"$uuid\"
  done
  jq -nc --arg t "$title" --arg id "$id" '{type: "ai-title", aiTitle: $t, sessionId: $id}' >>"$f"
  touch -d "$age" "$f"
}

session "$home/dev/shop" "Checkout total rounding fix" "2026-10-25 17:35" \
  "Fixed: the total is now rounded once, after the discount, instead of per line item. test_cart_totals.py covers 3 × 9.99 with 15% off (expects 25.47), and all 214 tests pass." \
  "the cart total is off by one cent when a 15% discount applies" \
  "add a test for 3 items at 9.99 with the discount" \
  "do other discount types round per line too?" \
  "ok, commit it"

session "$home/dev/shop/backend" "Order export walkthrough" "2026-10-24 16:10" \
  "Failed rows go to exports/failed/<date>.csv and the job logs a warning, but nothing retries them. The next nightly run starts after the last successful order ID, so failed rows stay skipped until someone runs bin/export --since." \
  "how does the nightly order export work?" \
  "where do failed rows end up?"

session "$home/dev/blog" "RSS feed" "2026-10-23 21:02" \
  "Added /feed.xml with the 20 newest published posts. Drafts and scheduled posts are left out. The W3C feed validator reports no errors." \
  "add an RSS feed for posts" \
  "only published posts, newest 20"

session "$home/dev/api-gateway" "Rate limiter returning 500s" "2026-10-22 11:47" \
  "The limiter threw when Redis was unreachable, which surfaced as a 500. It now fails open and logs the Redis error, so requests pass without limits during an outage instead of failing." \
  "the rate limiter returns 500 instead of 429 sometimes" \
  "what happens when redis is down?"

session "$home/dotfiles" "Neovim LSP setup for Go" "2026-10-21 09:15" \
  "gopls is configured through lspconfig with gofumpt formatting on save. Run :checkhealth lsp if it doesn't attach." \
  "set up gopls in my neovim config" \
  "format on save with gofumpt"

session "$home/dev/shop/frontend" "Product page image lazy loading" "2026-10-20 14:30" \
  "Images below the fold now use loading=\"lazy\" with explicit width and height, so the layout doesn't shift while they load." \
  "product pages load all 40 images up front, can we lazy load?"

session "$home/dev/blog" "Dark mode toggle" "2026-10-18 19:05" \
  "The toggle stores the choice in localStorage and falls back to prefers-color-scheme when nothing is stored." \
  "add a dark mode toggle to the header"

session "$home/notes" "Weekly report draft" "2026-10-17 08:40" \
  "Draft is in weekly/2026-42.md: three shipped items, one blocked on the payment provider, and next week's focus." \
  "draft my weekly report from this week's commits"

jq -n --arg h "$home" '{hasCompletedOnboarding: true, theme: "dark", projects: ([$h + "/dev/shop", $h + "/dev/shop/backend", $h + "/dev/shop/frontend", $h + "/dev/blog", $h + "/dev/api-gateway", $h + "/dotfiles", $h + "/notes"] | map({key: ., value: {hasTrustDialogAccepted: true}}) | from_entries)}' >"${1}/.claude.json"
