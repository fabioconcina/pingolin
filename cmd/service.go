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

const (
	daemonUnitName = "pingolin.service"
	daemonUnitPath = "/etc/systemd/system/" + daemonUnitName
	webUnitName    = "pingolin-web.service"
	webUnitPath    = "/etc/systemd/system/" + webUnitName
	sudoersPath    = "/etc/sudoers.d/pingolin"
)

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

	// Daemon unit
	daemonUnit := fmt.Sprintf(`[Unit]
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

	if err := os.WriteFile(daemonUnitPath, []byte(daemonUnit), 0o644); err != nil {
		return fmt.Errorf("writing daemon unit file: %w", err)
	}
	fmt.Printf("Wrote %s\n", daemonUnitPath)

	// Web unit
	webUnit := fmt.Sprintf(`[Unit]
Description=pingolin web dashboard
After=pingolin.service
Wants=pingolin.service

[Service]
Type=simple
User=%s
ExecStart=%s web
Restart=on-failure
RestartSec=5
AmbientCapabilities=CAP_NET_RAW

[Install]
WantedBy=multi-user.target
`, username, binPath)

	if err := os.WriteFile(webUnitPath, []byte(webUnit), 0o644); err != nil {
		return fmt.Errorf("writing web unit file: %w", err)
	}
	fmt.Printf("Wrote %s\n", webUnitPath)

	// Sudoers drop-in for passwordless service management
	sudoers := fmt.Sprintf(`# Allow %s to manage pingolin services without a password
%s ALL=(root) NOPASSWD: /usr/bin/systemctl stop pingolin.service, /usr/bin/systemctl start pingolin.service, /usr/bin/systemctl restart pingolin.service
%s ALL=(root) NOPASSWD: /usr/bin/systemctl stop pingolin-web.service, /usr/bin/systemctl start pingolin-web.service, /usr/bin/systemctl restart pingolin-web.service
`, username, username, username)

	if err := os.WriteFile(sudoersPath, []byte(sudoers), 0o440); err != nil {
		return fmt.Errorf("writing sudoers file: %w", err)
	}
	fmt.Printf("Wrote %s\n", sudoersPath)

	for _, cmdArgs := range [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", daemonUnitName},
		{"systemctl", "start", daemonUnitName},
		{"systemctl", "enable", webUnitName},
		{"systemctl", "start", webUnitName},
	} {
		out, err := exec.Command(cmdArgs[0], cmdArgs[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %s", cmdArgs, string(out))
		}
	}

	fmt.Println("Services installed and started.")
	fmt.Printf("  Daemon:  http://localhost (ICMP/DNS/HTTP probing)\n")
	fmt.Printf("  Web:     http://0.0.0.0:8080 (dashboard)\n")
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

	for _, cmdArgs := range [][]string{
		{"systemctl", "stop", webUnitName},
		{"systemctl", "disable", webUnitName},
		{"systemctl", "stop", daemonUnitName},
		{"systemctl", "disable", daemonUnitName},
	} {
		out, err := exec.Command(cmdArgs[0], cmdArgs[1:]...).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s", cmdArgs, string(out))
		}
	}

	for _, path := range []string{webUnitPath, daemonUnitPath, sudoersPath} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "removing %s: %v\n", path, err)
		}
	}

	exec.Command("systemctl", "daemon-reload").Run()

	fmt.Println("Services uninstalled.")
	return nil
}

func runServiceStatus(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("systemd services are only supported on Linux")
	}

	for _, unit := range []string{daemonUnitName, webUnitName} {
		c := exec.Command("systemctl", "status", unit)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Run()
		fmt.Println()
	}
	return nil
}

func runServiceLogs(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("systemd services are only supported on Linux")
	}

	c := exec.Command("journalctl", "-u", daemonUnitName, "-u", webUnitName, "-n", "50", "--no-pager")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Run()
	return nil
}
