#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <tag> <owner/repo>" >&2
  exit 1
fi

tag="$1"
repo="$2"
formula_path="Formula/viola.rb"
source_url="https://github.com/${repo}/archive/refs/tags/${tag}.tar.gz"
tmp_tarball="$(mktemp)"

cleanup() {
  rm -f "$tmp_tarball"
}
trap cleanup EXIT

curl -fsSL "$source_url" -o "$tmp_tarball"

sha="$(shasum -a 256 "$tmp_tarball" | awk '{print $1}')"

cat >"$formula_path" <<EOF
class Viola < Formula
  desc "CLI for reading FirenzeViola news"
  homepage "https://github.com/${repo}"
  url "${source_url}"
  sha256 "${sha}"
  license "MIT"
  head "https://github.com/${repo}.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args, "./cmd/viola"
  end

  test do
    assert_match "viola reads FirenzeViola news", shell_output("#{bin}/viola help")
  end
end
EOF
