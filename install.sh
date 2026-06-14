#!/bin/sh
# Install vd from GitHub releases.
#   curl -fsSL https://raw.githubusercontent.com/vanducng/vd-cli/main/install.sh | sh
# Env: VD_INSTALL_DIR (default ~/.local/bin), VD_VERSION (default latest).
set -eu

main() {
  REPO="vanducng/vd-cli"
  BIN="vd"
  INSTALL_DIR="${VD_INSTALL_DIR:-$HOME/.local/bin}"
  VERSION="${VD_VERSION:-}"

  command -v curl >/dev/null 2>&1 || err "curl is required"
  command -v tar >/dev/null 2>&1 || err "tar is required"
  if command -v shasum >/dev/null 2>&1; then
    SHA_CMD="shasum -a 256"
  elif command -v sha256sum >/dev/null 2>&1; then
    SHA_CMD="sha256sum"
  else
    err "shasum or sha256sum is required for checksum verification"
  fi

  previous_bin="$(command -v "$BIN" 2>/dev/null || true)"

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin|linux) ;;
    *) err "unsupported OS: $os (use the Windows zip from GitHub releases)" ;;
  esac

  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="x86_64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) err "unsupported architecture: $arch" ;;
  esac

  if [ -z "$VERSION" ]; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
      sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
    [ -n "$VERSION" ] || err "could not resolve latest release tag"
  fi

  asset="${BIN}_${os}_${arch}.tar.gz"
  base="https://github.com/$REPO/releases/download/$VERSION"
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  printf 'downloading %s %s (%s/%s)...\n' "$BIN" "$VERSION" "$os" "$arch"
  curl -fsSL -o "$tmp/$asset" "$base/$asset" || err "download failed: $base/$asset"
  curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || err "checksums.txt download failed"

  want="$(awk -v a="$asset" '$2 == a {print tolower($1)}' "$tmp/checksums.txt")"
  [ -n "$want" ] || err "no checksum entry for $asset in checksums.txt"
  got="$($SHA_CMD "$tmp/$asset" | awk '{print $1}')"
  [ "$got" = "$want" ] || err "checksum mismatch for $asset (got $got, want $want)"

  tar -xzf "$tmp/$asset" -C "$tmp" "$BIN"
  mkdir -p "$INSTALL_DIR"
  staged="$INSTALL_DIR/.$BIN.tmp.$$"
  cp "$tmp/$BIN" "$staged"
  chmod 0755 "$staged"
  mv "$staged" "$INSTALL_DIR/$BIN"

  installed="$INSTALL_DIR/$BIN"
  printf 'installed %s %s -> %s\n' "$BIN" "$VERSION" "$installed"
  post_install_path_notice "$BIN" "$INSTALL_DIR" "$installed" "$previous_bin"
}

post_install_path_notice() {
  bin="$1"
  install_dir="$2"
  installed="$3"
  previous_bin="$4"

  case ":$PATH:" in
    *":$install_dir:"*) ;;
    *) printf 'note: %s is not on your PATH\n' "$install_dir"; return ;;
  esac

  current_bin="$(command -v "$bin" 2>/dev/null || true)"
  if [ -z "$current_bin" ]; then
    printf 'note: %s is on PATH, but %s does not resolve yet; open a new shell or run %s directly\n' "$install_dir" "$bin" "$installed"
    return
  fi
  if [ "$current_bin" != "$installed" ]; then
    printf 'note: %s resolves to %s, not %s\n' "$bin" "$current_bin" "$installed"
    printf 'note: move %s earlier in PATH or run %s directly\n' "$install_dir" "$installed"
    return
  fi
  if [ -n "$previous_bin" ] && [ "$previous_bin" != "$installed" ]; then
    printf 'note: %s previously resolved to %s\n' "$bin" "$previous_bin"
    printf 'note: if this shell still runs the old binary, refresh its command cache with: hash -r (sh/bash) or rehash (zsh)\n'
  fi
}

err() { printf 'error: %s\n' "$1" >&2; exit 1; }

main "$@"
