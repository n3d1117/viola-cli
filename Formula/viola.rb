class Viola < Formula
  desc "CLI for reading FirenzeViola news"
  homepage "https://github.com/n3d1117/viola-cli"
  url "https://github.com/n3d1117/viola-cli/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "c4c09eabfffef0efd1c591acbe4feadd6e572763506870729bfbf8ee0904554c"
  license "MIT"
  head "https://github.com/n3d1117/viola-cli.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args, "./cmd/viola"
  end

  test do
    assert_match "viola reads FirenzeViola news", shell_output("#{bin}/viola help")
  end
end
