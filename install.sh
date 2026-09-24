#!/bin/sh
# Installs the drg binary from a GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/IgorBolotnikov/DRUDGE/main/install.sh | sh
#
# Environment:
#   DRG_VERSION      release tag to install, e.g. v0.1.0. Defaults to the latest release.
#   DRG_INSTALL_DIR  directory to put drg in. Defaults to /usr/local/bin for root and ~/.local/bin for everyone else.
#   DRG_BASE_URL     where the release files live. Defaults to the GitHub release of DRG_VERSION.
set -eu

repo="IgorBolotnikov/DRUDGE"
binary="drg"

fail() {
	echo "Error: $*" >&2
	exit 1
}

detect_os() {
	case "$(uname -s)" in
	Linux) echo linux ;;
	Darwin) echo darwin ;;
	*) fail "drg runs on Linux and macOS only, got $(uname -s). On Windows, run this script inside WSL." ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo amd64 ;;
	aarch64 | arm64) echo arm64 ;;
	*) fail "drg is built for amd64 and arm64 only, got $(uname -m)." ;;
	esac
}

download() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL -o "$2" "$1" || fail "could not download $1"
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$2" "$1" || fail "could not download $1"
	else
		fail "curl or wget is needed to download drg."
	fi
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d ' ' -f 1
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d ' ' -f 1
	else
		fail "sha256sum or shasum is needed to check the download."
	fi
}

os="$(detect_os)"
arch="$(detect_arch)"
archive="${binary}_${os}_${arch}.tar.gz"

version="${DRG_VERSION:-latest}"
if [ "$version" = "latest" ]; then
	default_base_url="https://github.com/$repo/releases/latest/download"
else
	default_base_url="https://github.com/$repo/releases/download/$version"
fi
base_url="${DRG_BASE_URL:-$default_base_url}"

if [ "$(id -u)" -eq 0 ]; then
	default_install_dir="/usr/local/bin"
else
	default_install_dir="$HOME/.local/bin"
fi
install_dir="${DRG_INSTALL_DIR:-$default_install_dir}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "Downloading $archive ($version)"
download "$base_url/$archive" "$tmp_dir/$archive"
download "$base_url/checksums.txt" "$tmp_dir/checksums.txt"

want_sum="$(grep " $archive\$" "$tmp_dir/checksums.txt" | cut -d ' ' -f 1)"
[ -n "$want_sum" ] || fail "checksums.txt has no entry for $archive."
got_sum="$(sha256 "$tmp_dir/$archive")"
[ "$got_sum" = "$want_sum" ] || fail "checksum of $archive is $got_sum, expected $want_sum."

tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" "$binary"
mkdir -p "$install_dir"
mv "$tmp_dir/$binary" "$install_dir/$binary"
chmod 755 "$install_dir/$binary"

echo "Installed $("$install_dir/$binary" --version) to $install_dir/$binary"
case ":$PATH:" in
*":$install_dir:"*) ;;
*) echo "$install_dir is not on your PATH. Add it to your shell profile to run drg." ;;
esac
echo "Run drg setup to finish."
