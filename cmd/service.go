package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage systemd service",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and start pingolin as a systemd service",
	RunE:  runServiceInstall,
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Stop and remove the pingolin systemd service",
	RunE:  runServiceUninstall,
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show service status",
	RunE:  runServiceStatus,
}

var serviceLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show service logs",
	RunE:  runServiceLogs,
}

func init() {
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(serviceLogsCmd)
}

const unitName = "pingolin.service"
const unitPath = "/etc/systemd/system/" + unitName

func runServiceInstall(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("systemd services are only supported on Linux")
	}

	if os.Geteuid() != 0 {
		return fmt.Errorf("must be run as root (use sudo)")
	}

	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable path: %w", err)
	}
	binPath, err = filepath.EvalSymlinks(binPath)
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	username := os.Getenv("SUDO_USER")
	if username == "" {
		u, err := user.Current()
		if err != nil {
			return fmt.Errorf("determining user: %w", err)
		}
		username = u.Username
	}

	unit := fmt.Sprintf(`[Unit]
Description=pingolin internet connection monitor
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
ExecStart=%s daemon
Restart=on-failure
RestartSec=5
AmbientCapabilities=CAP_NET_RAW

[Install]
WantedBy=multi-user.target
`, username, binPath)

	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("writing unit file: %w", err)
	}
	fmt.Printf("Wrote %s\n", unitPath)

	for _, args := range [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", unitName},
		{"systemctl", "start", unitName},
	} {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %s", args, string(out))
		}
	}

	fmt.Println("Service installed and started.")
	fmt.Printf("  Status:  sudo pingolin service status\n")
	fmt.Printf("  Logs:    sudo pingolin service logs\n")
	fmt.Printf("  Remove:  sudo pingolin service uninstall\n")
	return nil
}

func runServiceUninstall(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("systemd services are only supported on Linux")
	}

	if os.Geteuid() != 0 {
		return fmt.Errorf("must be run as root (use sudo)")
	}

	for _, args := range [][]string{
		{"systemctl", "stop", unitName},
		{"systemctl", "disable", unitName},
	} {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s", args, string(out))
		}
	}

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing unit file: %w", err)
	}

	exec.Command("systemctl", "daemon-reload").Run()

	fmt.Println("Service uninstalled.")
	return nil
}

func runServiceStatus(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("systemd services are only supported on Linux")
	}

	c := exec.Command("systemctl", "status", unitName)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Run()
	return nil
}

func runServiceLogs(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("systemd services are only supported on Linux")
	}

	c := exec.Command("journalctl", "-u", unitName, "-n", "50", "--no-pager")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Run()
	return nil
}
