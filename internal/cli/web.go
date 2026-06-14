package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/vanducng/vd-cli/v2/internal/inventory"
	"github.com/vanducng/vd-cli/v2/internal/ui/web"
)

func newWebCmd() *cobra.Command {
	var (
		port      int
		host      string
		noBrowser bool
	)
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Launch a local web UI to review skills, agents, and hooks",
		Long: `Start a localhost-only web server with a browsable inventory of the skills
vd tracks (with drift status), assets discovered under ~/.claude, and the
registered Claude hooks. Read-only.

This is the web frontend; tui and desktop frontends share the same backend.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWeb(cmd, host, port, noBrowser)
		},
	}
	f := cmd.Flags()
	f.IntVar(&port, "port", 7777, "Port to listen on")
	f.StringVar(&host, "host", "127.0.0.1", "Host to bind (loopback only)")
	f.BoolVar(&noBrowser, "no-browser", false, "Do not open a browser")
	return cmd
}

func runWeb(cmd *cobra.Command, host string, port int, noBrowser bool) error {
	if !isLoopback(host) {
		return fmt.Errorf("refusing to bind non-loopback host %q — vd web is a local review tool", host)
	}
	root, err := resolveRepoRoot(flagRoot)
	if err != nil {
		return err
	}
	claudeHome, err := claudeDir()
	if err != nil {
		return err
	}
	srv, err := web.NewServer(inventory.NewService(root, claudeHome))
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w (try --port)", addr, err)
	}

	httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
	url := "http://" + addr
	if !flagQuiet {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "vd web serving %s (Ctrl-C to stop)\n", url)
	}
	if !noBrowser {
		openBrowser(url) // best-effort; never fatal
	}

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.Serve(ln) }()

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func claudeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// openBrowser opens url in the default browser; any failure is ignored.
func openBrowser(url string) {
	var bin string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		bin, args = "open", []string{url}
	case "windows":
		bin, args = "cmd", []string{"/c", "start", url}
	default:
		bin, args = "xdg-open", []string{url}
	}
	_ = exec.Command(bin, args...).Start()
}
