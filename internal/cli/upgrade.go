package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vanducng/vd-cli/v2/internal/version"
)

const (
	upgradeRepo   = "vanducng/vd-cli"
	upgradeBinary = "vd"
	brewTap       = "vanducng/tap"
)

func newUpgradeCmd() *cobra.Command {
	var (
		checkOnly bool
		target    string
	)

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade vd to the latest release",
		Long: `Download the latest vd release from GitHub and replace the running binary
in place (verified against the published checksums).

Homebrew installs are not self-replaced; upgrade those with
'brew upgrade vanducng/tap/vd' (run 'brew trust vanducng/tap' first if the
tap is untrusted).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			return runUpgrade(ctx, cmd.OutOrStdout(), checkOnly, target)
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only report whether a newer version exists; do not install")
	cmd.Flags().StringVar(&target, "version", "", "Install a specific version tag (e.g. v2.3.0) instead of the latest")
	return cmd
}

func runUpgrade(ctx context.Context, w io.Writer, checkOnly bool, target string) error {
	current := version.Version
	client := &http.Client{Timeout: 5 * time.Minute}

	latest := strings.TrimSpace(target)
	if latest == "" {
		tag, err := fetchLatestReleaseTag(ctx, client)
		if err != nil {
			return err
		}
		latest = tag
	}
	updateAvailable := normalizeUpgradeTag(current) != normalizeUpgradeTag(latest)

	if checkOnly {
		_, _ = fmt.Fprintf(w, "current: %s\nlatest:  %s\nupdate available: %t\n", current, latest, updateAvailable)
		return nil
	}
	if target == "" && !updateAvailable {
		_, _ = fmt.Fprintf(w, "vd is already up to date (%s)\n", current)
		return nil
	}
	if exe, _ := os.Executable(); isHomebrewPath(resolveSymlinks(exe)) {
		return errHomebrewManaged()
	}

	asset := upgradeAssetName(runtime.GOOS, runtime.GOARCH)
	if asset == "" {
		return fmt.Errorf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	base := fmt.Sprintf("https://github.com/%s/releases/download/%s", upgradeRepo, latest)

	if !flagQuiet {
		_, _ = fmt.Fprintf(w, "downloading %s %s...\n", upgradeBinary, latest)
	}
	archive, err := httpDownload(ctx, client, base+"/"+asset)
	if err != nil {
		return fmt.Errorf("download %s: %w (check that release %s exists)", asset, err, latest)
	}
	sums, err := httpDownload(ctx, client, base+"/checksums.txt")
	if err != nil {
		return fmt.Errorf("download checksums.txt: %w", err)
	}
	want := checksumFor(string(sums), asset)
	if want == "" {
		return fmt.Errorf("no checksum entry for %s in checksums.txt", asset)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(archive)); got != want {
		return fmt.Errorf("checksum mismatch for %s (got %s, want %s)", asset, got, want)
	}

	bin, err := extractUpgradeBinary(archive, runtime.GOOS)
	if err != nil {
		return err
	}
	installedPath, err := replaceRunningExecutable(bin)
	if err != nil {
		return err
	}

	if !flagQuiet {
		_, _ = fmt.Fprintf(w, "upgraded vd %s -> %s\n%s\n", current, latest, installedPath)
	}
	return nil
}

func fetchLatestReleaseTag(ctx context.Context, client *http.Client) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", upgradeRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "vd-upgrade/"+version.Version)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot reach GitHub: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github returned status %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode github response: %w", err)
	}
	if payload.TagName == "" {
		return "", fmt.Errorf("github response missing tag_name")
	}
	return payload.TagName, nil
}

func httpDownload(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "vd-upgrade/"+version.Version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// upgradeAssetName mirrors the goreleaser archive name_template:
// vd_<os>_<x86_64|arch>.{tar.gz|zip}.
func upgradeAssetName(goos, goarch string) string {
	var arch string
	switch goarch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "arm64"
	default:
		return ""
	}
	switch goos {
	case "darwin", "linux":
		return fmt.Sprintf("%s_%s_%s.tar.gz", upgradeBinary, goos, arch)
	case "windows":
		return fmt.Sprintf("%s_%s_%s.zip", upgradeBinary, goos, arch)
	default:
		return ""
	}
}

func upgradeBinaryName(goos string) string {
	if goos == "windows" {
		return upgradeBinary + ".exe"
	}
	return upgradeBinary
}

func checksumFor(checksums, asset string) string {
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return strings.ToLower(fields[0])
		}
	}
	return ""
}

func normalizeUpgradeTag(tag string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(tag)), "v")
}

func extractUpgradeBinary(archive []byte, goos string) ([]byte, error) {
	want := upgradeBinaryName(goos)
	if goos == "windows" {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if path.Base(f.Name) == want {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer func() { _ = rc.Close() }()
				return io.ReadAll(rc)
			}
		}
		return nil, fmt.Errorf("%s not found in archive", want)
	}

	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if path.Base(hdr.Name) == want {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%s not found in archive", want)
}

// replaceRunningExecutable atomically swaps the running binary for newBin and
// returns the installed path. Homebrew-managed installs are refused.
func replaceRunningExecutable(newBin []byte) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe = resolveSymlinks(exe)
	if isHomebrewPath(exe) {
		return exe, errHomebrewManaged()
	}

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".vd-upgrade-*")
	if err != nil {
		return exe, upgradePermHint(err, dir)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(newBin); err != nil {
		_ = tmp.Close()
		return exe, err
	}
	if err := tmp.Close(); err != nil {
		return exe, err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return exe, err
	}

	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return exe, upgradePermHint(err, dir)
		}
		if err := os.Rename(tmpName, exe); err != nil {
			_ = os.Rename(old, exe)
			return exe, err
		}
		_ = os.Remove(old)
		return exe, nil
	}
	if err := os.Rename(tmpName, exe); err != nil {
		return exe, upgradePermHint(err, dir)
	}
	return exe, nil
}

func resolveSymlinks(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return p
}

func isHomebrewPath(p string) bool {
	return strings.Contains(p, "/Cellar/") || strings.Contains(p, "/homebrew/")
}

// errHomebrewManaged includes the trust step: Homebrew >=5.x refuses
// formulae from untrusted third-party taps.
func errHomebrewManaged() error {
	return fmt.Errorf("vd was installed via Homebrew; upgrade with: brew upgrade %s/%s (if brew refuses the untrusted tap, first run: brew trust %s)", brewTap, upgradeBinary, brewTap)
}

func upgradePermHint(err error, dir string) error {
	if os.IsPermission(err) {
		return fmt.Errorf("cannot write to %s: %w (re-run with elevated permissions or reinstall via your package manager)", dir, err)
	}
	return err
}
