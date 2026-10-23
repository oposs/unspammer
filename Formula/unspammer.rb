# Homebrew formula for unspammer. This repository is its own tap.
#
# The release build rewrites the version line, the four sha256 lines by their
# marker comments and the bottle block between the BOTTLE markers. Keep the
# markers, and keep each url line directly above its sha256 line.
class Unspammer < Formula
  desc "IMAP robot that acts on mail arriving in watched folders"
  homepage "https://github.com/oposs/unspammer"
  version "0.4.0"
  license "GPL-3.0"

  # BOTTLE-START
  # BOTTLE-END

  on_macos do
    on_arm do
      url "https://github.com/oposs/unspammer/releases/download/v#{version}/unspammer-#{version}-aarch64-apple-darwin.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000" # mac-arm
    end
    on_intel do
      url "https://github.com/oposs/unspammer/releases/download/v#{version}/unspammer-#{version}-x86_64-apple-darwin.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000" # mac-x86
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/oposs/unspammer/releases/download/v#{version}/unspammer-#{version}-x86_64-unknown-linux-musl.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000" # linux-x86
    end
    on_arm do
      url "https://github.com/oposs/unspammer/releases/download/v#{version}/unspammer-#{version}-aarch64-unknown-linux-musl.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000" # linux-arm
    end
  end

  def install
    bin.install "unspammer"
    Dir["man/*.[1-9]"].each { |page| (man/"man#{page[-1]}").install page }
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/unspammer -version")
  end
end
