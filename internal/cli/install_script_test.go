package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallScriptWarnsWhenExistingVDBinaryWasElsewhere(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is for macOS/Linux")
	}
	asset := upgradeAssetName(runtime.GOOS, runtime.GOARCH)
	if asset == "" || !strings.HasSuffix(asset, ".tar.gz") {
		t.Skipf("install.sh does not support %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	tmp := t.TempDir()
	installDir := filepath.Join(tmp, "local", "bin")
	oldBinDir := filepath.Join(tmp, "homebrew", "bin")
	fakeBinDir := filepath.Join(tmp, "fakebin")
	for _, dir := range []string{installDir, oldBinDir, fakeBinDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldVD := filepath.Join(oldBinDir, "vd")
	if err := os.WriteFile(oldVD, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	archivePath := filepath.Join(tmp, asset)
	archive := buildInstallScriptArchive(t)
	if err := os.WriteFile(archivePath, archive, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archive)
	checksumsPath := filepath.Join(tmp, "checksums.txt")
	checksums := fmt.Sprintf("%x  %s\n", sum, asset)
	if err := os.WriteFile(checksumsPath, []byte(checksums), 0o644); err != nil {
		t.Fatal(err)
	}

	fakeCurl := filepath.Join(fakeBinDir, "curl")
	if err := os.WriteFile(fakeCurl, []byte(`#!/bin/sh
set -eu
if [ "$1" = "-fsSL" ] && [ "$2" = "-o" ]; then
  out="$3"
  url="$4"
  case "$url" in
    *checksums.txt) cp "$VD_TEST_CHECKSUMS" "$out" ;;
    *.tar.gz) cp "$VD_TEST_ARCHIVE" "$out" ;;
    *) exit 64 ;;
  esac
  exit 0
fi
exit 64
`), 0o755); err != nil {
		t.Fatal(err)
	}

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	cmd := exec.Command("sh", filepath.Join(repoRoot, "install.sh"))
	cmd.Env = append(os.Environ(),
		"PATH="+strings.Join([]string{fakeBinDir, installDir, oldBinDir, os.Getenv("PATH")}, string(os.PathListSeparator)),
		"VD_INSTALL_DIR="+installDir,
		"VD_VERSION=v9.9.9",
		"VD_TEST_ARCHIVE="+archivePath,
		"VD_TEST_CHECKSUMS="+checksumsPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, out)
	}

	got := string(out)
	for _, want := range []string{
		"installed vd v9.9.9 -> " + filepath.Join(installDir, "vd"),
		"note: vd previously resolved to " + oldVD,
		"hash -r (sh/bash) or rehash (zsh)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("install.sh output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func buildInstallScriptArchive(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	writeTar(t, tw, "vd", []byte("#!/bin/sh\nprintf 'vd test\\n'\n"))
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
