package cli

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nakagami-306/orbit/internal/api"
	"github.com/spf13/cobra"
)

func newUICmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Start the Web UI server",
		Long:  "Start the Orbit Web UI server in the background and open the browser.",
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetInt("port")
			noBrowser, _ := cmd.Flags().GetBool("no-browser")
			daemon, _ := cmd.Flags().GetBool("daemon")

			// Check if already running
			if pid, addr, err := api.ReadPidFile(); err == nil {
				if isProcessRunning(pid) {
					url := "http://" + addr
					fmt.Printf("Orbit UI is already running at %s (PID %d)\n", url, pid)
					if !noBrowser {
						openBrowser(url)
					}
					return nil
				}
				// Stale PID file
				api.RemovePidFile()
			}

			if daemon {
				return runDaemon(port)
			}
			return spawnDaemon(port, noBrowser)
		},
	}

	cmd.Flags().IntP("port", "p", 19840, "Port to listen on")
	cmd.Flags().Bool("no-browser", false, "Don't open browser automatically")
	cmd.Flags().Bool("daemon", false, "Run as daemon process (internal)")
	cmd.Flags().MarkHidden("daemon")

	cmd.AddCommand(newUIStopCmd())
	cmd.AddCommand(newUIStatusCmd())

	return cmd
}

// runDaemon starts the server in the current process and blocks.
// Called when --daemon flag is set (by the spawned child process).
func runDaemon(port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	srv, err := api.NewServer(addr)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	actualAddr, err := srv.Start()
	if err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	pid := os.Getpid()
	if err := api.WritePidFile(pid, actualAddr); err != nil {
		return fmt.Errorf("write PID file: %w", err)
	}

	log.Printf("Orbit UI daemon started at http://%s (PID %d)", actualAddr, pid)

	// Block forever — daemon process
	select {}
}

// spawnDaemon re-executes this binary with --daemon in a detached process,
// waits for the PID file, opens the browser, and returns immediately.
func spawnDaemon(port int, noBrowser bool) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	logFile, err := api.OpenLogFile()
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	cmd := exec.Command(exe, "ui", "--daemon", "--port", strconv.Itoa(port))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	setDetachAttrs(cmd)

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start daemon: %w", err)
	}

	// Release the child process handle — it's fully detached
	cmd.Process.Release()
	logFile.Close()

	// Wait for the daemon to write its PID file (confirms successful startup)
	addr, err := waitForPidFile(3 * time.Second)
	if err != nil {
		return fmt.Errorf("daemon failed to start (check %s): %w", api.LogFilePath(), err)
	}

	url := "http://" + addr
	fmt.Printf("Orbit UI started at %s\n", url)

	if !noBrowser {
		openBrowser(url)
	}

	return nil
}

// waitForPidFile polls for the PID file to appear within the given timeout.
func waitForPidFile(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, addr, err := api.ReadPidFile(); err == nil {
			return addr, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", fmt.Errorf("timeout waiting for server startup")
}

func newUIStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the Web UI server",
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, addr, err := api.ReadPidFile()
			if err != nil {
				return fmt.Errorf("UI server is not running (no PID file)")
			}

			if !isProcessRunning(pid) {
				api.RemovePidFile()
				fmt.Println("UI server was not running (stale PID file removed)")
				return nil
			}

			proc, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("find process: %w", err)
			}

			if err := proc.Kill(); err != nil {
				return fmt.Errorf("kill process: %w", err)
			}

			api.RemovePidFile()
			fmt.Printf("Orbit UI stopped (was running at http://%s, PID %d)\n", addr, pid)
			return nil
		},
	}
}

func newUIStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check Web UI server status",
		Run: func(cmd *cobra.Command, args []string) {
			pid, addr, err := api.ReadPidFile()
			if err != nil {
				fmt.Println("Orbit UI is not running")
				return
			}

			if isProcessRunning(pid) {
				fmt.Printf("Orbit UI is running at http://%s (PID %d)\n", addr, pid)
			} else {
				api.RemovePidFile()
				fmt.Println("Orbit UI is not running (stale PID file removed)")
			}
		},
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

func isProcessRunning(pid int) bool {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH").Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), strconv.Itoa(pid))
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
