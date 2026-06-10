package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func TestUpgradeAssetName(t *testing.T) {
	tests := []struct {
		goos, goarch, want string
	}{
		{"darwin", "arm64", "vd_darwin_arm64.tar.gz"},
		{"darwin", "amd64", "vd_darwin_x86_64.tar.gz"},
		{"linux", "amd64", "vd_linux_x86_64.tar.gz"},
		{"linux", "arm64", "vd_linux_arm64.tar.gz"},
		{"windows", "amd64", "vd_windows_x86_64.zip"},
		{"windows", "arm64", "vd_windows_arm64.zip"},
		{"darwin", "386", ""},
		{"plan9", "amd64", ""},
	}
	for _, tt := range tests {
		if got := upgradeAssetName(tt.goos, tt.goarch); got != tt.want {
			t.Errorf("upgradeAssetName(%q,%q) = %q, want %q", tt.goos, tt.goarch, got, tt.want)
		}
	}
}

func TestChecksumFor(t *testing.T) {
	sums := "abc123  vd_darwin_arm64.tar.gz\ndef456  vd_linux_x86_64.tar.gz\n"
	if got := checksumFor(sums, "vd_darwin_arm64.tar.gz"); got != "abc123" {
		t.Errorf("checksumFor = %q, want abc123", got)
	}
	if got := checksumFor(sums, "vd_windows_arm64.zip"); got != "" {
		t.Errorf("checksumFor(missing) = %q, want empty", got)
	}
}

func TestNormalizeUpgradeTag(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"v2.3.0", "2.3.0"},
		{"V2.3.0", "2.3.0"},
		{" v2.3.0 ", "2.3.0"},
		{"2.3.0", "2.3.0"},
		{"dev", "dev"},
	} {
		if got := normalizeUpgradeTag(tt.in); got != tt.want {
			t.Errorf("normalizeUpgradeTag(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractUpgradeBinary_TarGz(t *testing.T) {
	want := []byte("#!fake vd binary contents")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	// A decoy file plus the real binary, to prove name matching works.
	writeTar(t, tw, "LICENSE", []byte("MIT"))
	writeTar(t, tw, "vd", want)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := extractUpgradeBinary(buf.Bytes(), "linux")
	if err != nil {
		t.Fatalf("extractUpgradeBinary: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("extracted = %q, want %q", got, want)
	}
}

func TestExtractUpgradeBinary_NotFound(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	writeTar(t, tw, "README.md", []byte("docs"))
	_ = tw.Close()
	_ = gz.Close()

	if _, err := extractUpgradeBinary(buf.Bytes(), "linux"); err == nil {
		t.Fatal("expected error when binary missing from archive")
	}
}

func TestIsHomebrewPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/opt/homebrew/bin/vd", true},
		{"/opt/homebrew/Cellar/vd/2.5.0/bin/vd", true},
		{"/usr/local/Cellar/vd/2.5.0/bin/vd", true},
		{"/usr/local/bin/vd", false},
		{"/Users/x/.local/bin/vd", false},
	}
	for _, tt := range tests {
		if got := isHomebrewPath(tt.path); got != tt.want {
			t.Errorf("isHomebrewPath(%q) = %t, want %t", tt.path, got, tt.want)
		}
	}
}

func TestErrHomebrewManaged_IncludesTapAndTrustHint(t *testing.T) {
	msg := errHomebrewManaged().Error()
	for _, want := range []string{"brew upgrade vanducng/tap/vd", "brew trust vanducng/tap"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
}

func writeTar(t *testing.T, tw *tar.Writer, name string, data []byte) {
	t.Helper()
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
}
