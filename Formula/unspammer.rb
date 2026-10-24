# Homebrew formula for unspammer. This repository is its own tap.
#
# The release build rewrites the version line, the four sha256 lines by their
# marker comments and the bottle block between the BOTTLE markers. Keep the
# markers, and keep each url line directly above its sha256 line.
class Unspammer < Formula
  desc "IMAP robot that acts on mail arriving in watched folders"
  homepage "https://github.com/oposs/unspammer"
  version "0.5.0"
  license "GPL-3.0"

  # BOTTLE-START
  bottle do
    root_url "https://github.com/oposs/unspammer/releases/download/v0.5.0"
    sha256 cellar: :any_skip_relocation, arm64_sonoma: "4043e3f4d3ab3cb3c9346e954f1126d04bd483f05ae859822dbc76f806a1fa67"
  end
  # BOTTLE-END

  on_macos do
    on_arm do
      url "https://github.com/oposs/unspammer/releases/download/v#{version}/unspammer-#{version}-aarch64-apple-darwin.tar.gz"
      sha256 "b62d77594304cceda88ef76499d43e2eb92a5e1b875e705ce98add65a952fd5c" # mac-arm
    end
    on_intel do
      url "https://github.com/oposs/unspammer/releases/download/v#{version}/unspammer-#{version}-x86_64-apple-darwin.tar.gz"
      sha256 "c8e0062be6b7d1cbd72da92030403469160df4db6d73397530a374f54db18857" # mac-x86
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/oposs/unspammer/releases/download/v#{version}/unspammer-#{version}-x86_64-unknown-linux-musl.tar.gz"
      sha256 "cbb7e197eb0494a35cb599bcaeca091cd4ee8a27ed6bc1e9e0f5f66a56ac8cc9" # linux-x86
    end
    on_arm do
      url "https://github.com/oposs/unspammer/releases/download/v#{version}/unspammer-#{version}-aarch64-unknown-linux-musl.tar.gz"
      sha256 "36692509c6eac6d2cdf95806e62eb39f8712b081064fb458d2a873a586d3b6c6" # linux-arm
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
