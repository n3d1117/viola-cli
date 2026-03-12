class Viola < Formula
  desc "CLI for reading FirenzeViola news"
  homepage "https://github.com/n3d1117/viola-cli"
  url "https://github.com/n3d1117/viola-cli.git",
      branch: "main",
      using: :git
  version "0.0.0"
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
