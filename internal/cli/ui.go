package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"

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

			// Check if already running
			if pid, addr, err := api.ReadPidFile(); err == nil {
				if isProcessRunning(pid) {
					fmt.Printf("Orbit UI is already running at http://%s (PID %d)\n", addr, pid)
					return nil
				}
				// Stale PID file
				api.RemovePidFile()
			}

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
				fmt.Fprintf(os.Stderr, "Warning: could not write PID file: %v\n", err)
			}

			url := "http://" + actualAddr
			fmt.Printf("Orbit UI started at %s (PID %d)\n", url, pid)

			if !noBrowser {
				openBrowser(url)
			}

			// Block — server runs in the foreground of this process
			// (launched as a detached process by the caller if background is desired)
			select {}
		},
	}

	cmd.Flags().IntP("port", "p", 19840, "Port to listen on")
	cmd.Flags().Bool("no-browser", false, "Don't open browser automatically")

	cmd.AddCommand(newUIStopCmd())
	cmd.AddCommand(newUIStatusCmd())

	return cmd
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
		// On Windows, os.FindProcess always succeeds; use tasklist to check
		out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH").Output()
		if err != nil {
			return false
		}
		return len(out) > 0 && indexBytes(out, byte(strconv.Itoa(pid)[0])) >= 0
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func indexBytes(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}
