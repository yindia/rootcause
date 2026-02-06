#!/bin/sh
set -e
# Based on godownloader output; refactored to match yindia/rootcause release asset names:
#   rootcause_<version>_<os>_<arch>.tar.gz (mac/linux) and .zip (windows)
# and checksums.txt present in the release.

usage() {
  this=$1
  cat <<EOF
$this: download binaries for yindia/rootcause

Usage: $this [-b] bindir [-d] [tag]
  -b sets bindir or installation directory, Defaults to ./bin
  -d turns on debug logging
   [tag] is a tag from
   https://github.com/yindia/rootcause/releases
   If tag is missing, then the latest will be used.
EOF
  exit 2
}

parse_args() {
  BINDIR=${BINDIR:-./bin}
  while getopts "b:dh?x" arg; do
    case "$arg" in
      b) BINDIR="$rootcauseRG" ;;
      d) log_set_priority 7 ;; # debug
      h | \?) usage "$0" ;;
      x) set -x ;;
    esac
  done
  shift $((OPTIND - 1))
  TAG=$1
}

execute() {
  tmpdir=$(mktemp -d)
  log_debug "downloading files into ${tmpdir}"

  http_download "${tmpdir}/${TARBALL}" "${TARBALL_URL}"
  http_download "${tmpdir}/${CHECKSUM}" "${CHECKSUM_URL}"
  hash_sha256_verify "${tmpdir}/${TARBALL}" "${tmpdir}/${CHECKSUM}"

  srcdir="${tmpdir}"
  (cd "${tmpdir}" && untar "${TARBALL}")

  test ! -d "${BINDIR}" && install -d "${BINDIR}"

  for binexe in $BINARIES; do
    if [ "$OS" = "windows" ]; then
      binexe="${binexe}.exe"
    fi

    # Install from extracted contents. If archives contain a folder, this still works.
    found="$(find "${srcdir}" -maxdepth 3 -type f -name "${binexe}" 2>/dev/null | head -n 1)"
    if [ -z "$found" ]; then
      log_crit "unable to find extracted binary '${binexe}' in archive"
      exit 1
    fi

    install "${found}" "${BINDIR}/"
    log_info "installed ${BINDIR}/${binexe}"
  done

  rm -rf "${tmpdir}"
}

get_binaries() {
  case "$PLATFORM" in
    darwin/amd64|darwin/arm64|linux/amd64|linux/arm64|windows/amd64|windows/arm64)
      BINARIES="rootcause"
      ;;
    *)
      log_crit "platform $PLATFORM is not supported. File an issue at https://github.com/${PREFIX}/issues/new"
      exit 1
      ;;
  esac
}

tag_to_version() {
  if [ -z "${TAG}" ]; then
    log_info "checking GitHub for latest tag"
  else
    log_info "checking GitHub for tag '${TAG}'"
  fi

  REALTAG=$(github_release "$OWNER/$REPO" "${TAG}") && true
  if test -z "$REALTAG"; then
    log_crit "unable to find '${TAG}' - use 'latest' or see https://github.com/${PREFIX}/releases for details"
    exit 1
  fi

  TAG="$REALTAG"
  VERSION=${TAG#v} # strip leading 'v'
}

adjust_format() {
  case ${OS} in
    windows) FORMAT=zip ;;
    *) FORMAT=tar.gz ;;
  esac
  true
}

cat /dev/null <<EOF
------------------------------------------------------------------------
https://github.com/client9/shlib - portable posix shell functions
Public domain - http://unlicense.org
------------------------------------------------------------------------
EOF

is_command() { command -v "$1" >/dev/null 2>&1; }

echoerr() { echo "$@" 1>&2; }

log_prefix() { echo "$0"; }

_logp=6
log_set_priority() { _logp="$1"; }

log_priority() {
  if test -z "$1"; then
    echo "$_logp"
    return
  fi
  [ "$1" -le "$_logp" ]
}

log_tag() {
  case $1 in
    0) echo "emerg" ;;
    1) echo "alert" ;;
    2) echo "crit" ;;
    3) echo "err" ;;
    4) echo "warning" ;;
    5) echo "notice" ;;
    6) echo "info" ;;
    7) echo "debug" ;;
    *) echo "$1" ;;
  esac
}

log_debug() { log_priority 7 || return 0; echoerr "$(log_prefix)" "$(log_tag 7)" "$@"; }
log_info()  { log_priority 6 || return 0; echoerr "$(log_prefix)" "$(log_tag 6)" "$@"; }
log_err()   { log_priority 3 || return 0; echoerr "$(log_prefix)" "$(log_tag 3)" "$@"; }
log_crit()  { log_priority 2 || return 0; echoerr "$(log_prefix)" "$(log_tag 2)" "$@"; }

uname_os() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$os" in
    cygwin_nt*|mingw*|msys_nt*) os="windows" ;;
  esac
  echo "$os"
}

uname_arch() {
  arch=$(uname -m)
  case $arch in
    x86_64) arch="amd64" ;;
    x86|i686|i386) arch="386" ;;
    aarch64|arm64) arch="arm64" ;;
    armv5*) arch="armv5" ;;
    armv6*) arch="armv6" ;;
    armv7*) arch="armv7" ;;
  esac
  echo "${arch}"
}

uname_os_check() {
  os=$(uname_os)
  case "$os" in
    darwin|dragonfly|freebsd|linux|android|nacl|netbsd|openbsd|plan9|solaris|windows) return 0 ;;
  esac
  log_crit "uname_os_check '$(uname -s)' got converted to '$os' which is not a GOOS value."
  return 1
}

uname_arch_check() {
  arch=$(uname_arch)
  case "$arch" in
    386|amd64|arm64|armv5|armv6|armv7|ppc64|ppc64le|mips|mipsle|mips64|mips64le|s390x|amd64p32) return 0 ;;
  esac
  log_crit "uname_arch_check '$(uname -m)' got converted to '$arch' which is not a GOARCH value."
  return 1
}

untar() {
  tarball=$1
  case "${tarball}" in
    *.tar.gz|*.tgz) tar --no-same-owner -xzf "${tarball}" ;;
    *.tar) tar --no-same-owner -xf "${tarball}" ;;
    *.zip) unzip -q "${tarball}" ;;
    *) log_err "untar unknown archive format for ${tarball}"; return 1 ;;
  esac
}

http_download_curl() {
  local_file=$1
  source_url=$2
  header=$3
  if [ -z "$header" ]; then
    code=$(curl -w '%{http_code}' -sSL -o "$local_file" "$source_url")
  else
    code=$(curl -w '%{http_code}' -sSL -H "$header" -o "$local_file" "$source_url")
  fi
  [ "$code" = "200" ] || return 1
  return 0
}

http_download_wget() {
  local_file=$1
  source_url=$2
  header=$3
  if [ -z "$header" ]; then
    wget -q -O "$local_file" "$source_url"
  else
    wget -q --header "$header" -O "$local_file" "$source_url"
  fi
}

http_download() {
  log_debug "http_download $2"
  if is_command curl; then
    http_download_curl "$@"
    return
  elif is_command wget; then
    http_download_wget "$@"
    return
  fi
  log_crit "http_download unable to find wget or curl"
  return 1
}

http_copy() {
  tmp=$(mktemp)
  http_download "${tmp}" "$1" "$2" || return 1
  body=$(cat "$tmp")
  rm -f "${tmp}"
  echo "$body"
}

github_release() {
  owner_repo=$1
  version=$2
  test -z "$version" && version="latest"
  giturl="https://github.com/${owner_repo}/releases/${version}"
  json=$(http_copy "$giturl" "Accept:application/json")
  test -z "$json" && return 1
  version=$(echo "$json" | tr -s '\n' ' ' | sed 's/.*"tag_name":"//' | sed 's/".*//')
  test -z "$version" && return 1
  echo "$version"
}

hash_sha256() {
  TARGET=${1:-/dev/stdin}
  if is_command gsha256sum; then
    gsha256sum "$TARGET" | awk '{print $1}'
  elif is_command sha256sum; then
    sha256sum "$TARGET" | awk '{print $1}'
  elif is_command shasum; then
    shasum -a 256 "$TARGET" 2>/dev/null | awk '{print $1}'
  elif is_command openssl; then
    openssl dgst -sha256 "$TARGET" | awk '{print $NF}'
  else
    log_crit "hash_sha256 unable to find command to compute sha-256 hash"
    return 1
  fi
}

hash_sha256_verify() {
  TARGET=$1
  checksums=$2
  if [ -z "$checksums" ]; then
    log_err "hash_sha256_verify checksum file not specified in arg2"
    return 1
  fi
  BASENAME=${TARGET##*/}

  # checksums.txt typically: "<hash>  <filename>"
  want=$(grep " ${BASENAME}\$" "${checksums}" 2>/dev/null | awk '{print $1}')
  if [ -z "$want" ]; then
    log_err "hash_sha256_verify unable to find checksum for '${TARGET}' in '${checksums}'"
    return 1
  fi

  got=$(hash_sha256 "$TARGET")
  if [ "$want" != "$got" ]; then
    log_err "hash_sha256_verify checksum for '$TARGET' did not verify ${want} vs $got"
    return 1
  fi
}

cat /dev/null <<EOF
------------------------------------------------------------------------
End of functions from https://github.com/client9/shlib
------------------------------------------------------------------------
EOF

PROJECT_NAME="rootcause"
OWNER="yindia"
REPO="rootcause"
BINARY="rootcause"
FORMAT="tar.gz"

OS=$(uname_os)
ARCH=$(uname_arch)
PREFIX="$OWNER/$REPO"

# use in logging routines
log_prefix() { echo "$PREFIX"; }

PLATFORM="${OS}/${ARCH}"
GITHUB_DOWNLOAD="https://github.com/${OWNER}/${REPO}/releases/download"

uname_os_check "$OS"
uname_arch_check "$ARCH"

parse_args "$@"
get_binaries
tag_to_version
adjust_format

log_info "found version: ${VERSION} for ${TAG}/${OS}/${ARCH}"

# ✅ matches your GitHub release assets:
# rootcause_0.0.1-beta_darwin_arm64.tar.gz etc.
NAME="${PROJECT_NAME}_${VERSION}_${OS}_${ARCH}"
TARBALL="${NAME}.${FORMAT}"
TARBALL_URL="${GITHUB_DOWNLOAD}/${TAG}/${TARBALL}"

CHECKSUM="checksums.txt"
CHECKSUM_URL="${GITHUB_DOWNLOAD}/${TAG}/${CHECKSUM}"

execute
