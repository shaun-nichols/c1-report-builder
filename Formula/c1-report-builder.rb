class C1ReportBuilder < Formula
  desc "ConductorOne Report Builder — custom reports for auditing ConductorOne data"
  homepage "https://github.com/shaun-nichols/c1-report-builder"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/shaun-nichols/c1-report-builder/releases/download/v#{version}/c1-report-builder-darwin-arm64"
      sha256 "PLACEHOLDER"

      def install
        bin.install "c1-report-builder-darwin-arm64" => "c1-report-builder"
      end
    else
      url "https://github.com/shaun-nichols/c1-report-builder/releases/download/v#{version}/c1-report-builder-darwin-amd64"
      sha256 "PLACEHOLDER"

      def install
        bin.install "c1-report-builder-darwin-amd64" => "c1-report-builder"
      end
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/shaun-nichols/c1-report-builder/releases/download/v#{version}/c1-report-builder-linux-arm64"
      sha256 "PLACEHOLDER"

      def install
        bin.install "c1-report-builder-linux-arm64" => "c1-report-builder"
      end
    else
      url "https://github.com/shaun-nichols/c1-report-builder/releases/download/v#{version}/c1-report-builder-linux-amd64"
      sha256 "PLACEHOLDER"

      def install
        bin.install "c1-report-builder-linux-amd64" => "c1-report-builder"
      end
    end
  end

  test do
    assert_match "c1-report-builder", shell_output("#{bin}/c1-report-builder --version")
  end
end
