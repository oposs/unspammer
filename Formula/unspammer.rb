# Homebrew formula for unspammer. This repository is its own tap.
#
# The release build rewrites the version line, the four sha256 lines by their
# marker comments and the bottle block between the BOTTLE markers. Keep the
# markers, and keep each url line directly above its sha256 line.
class Unspammer < Formula
  desc "IMAP robot that acts on mail arriving in watched folders"
  homepage "https://github.com/oposs/unspammer"
  version "0.5.1"
  license "GPL-3.0"

  # BOTTLE-START
  bottle do
    root_url "https://github.com/oposs/unspammer/releases/download/v0.5.1"
    sha256 cellar: :any_skip_relocation, arm64_monterey: "fb9354365209e5423ad76b046fc0eefaa1fe395ffbdc76fab2e42652e0f03fa7"
  end
  # BOTTLE-END

  on_macos do
    on_arm do
      url "https://github.com/oposs/unspammer/releases/download/v#{version}/unspammer-#{version}-aarch64-apple-darwin.tar.gz"
      sha256 "5313c7eb6fc8e07f34c095e0b8dd4c3aa7574785688e84d175aaa7e76308beab" # mac-arm
    end
    on_intel do
      url "https://github.com/oposs/unspammer/releases/download/v#{version}/unspammer-#{version}-x86_64-apple-darwin.tar.gz"
      sha256 "ef818c5184bddc3af88aa3e32255280d576ae04687dbfddf80fe9d7aef6fc19d" # mac-x86
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/oposs/unspammer/releases/download/v#{version}/unspammer-#{version}-x86_64-unknown-linux-musl.tar.gz"
      sha256 "004d0b1de7d3ab603ce6cd2b1126a54e2e8e05c1d196a6b59d6c3be4e62b3be4" # linux-x86
    end
    on_arm do
      url "https://github.com/oposs/unspammer/releases/download/v#{version}/unspammer-#{version}-aarch64-unknown-linux-musl.tar.gz"
      sha256 "14bebd59ed0f1ed9fb12db33f2a9a91f00f392cd95a7cdc298e20c4a4636cd5d" # linux-arm
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
