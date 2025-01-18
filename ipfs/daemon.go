package ipfs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
)

func (m *IPFSManager) Initialize() error {
	if m.initialized {
		return nil
	}

	if _, err := os.Stat(filepath.Join(m.DataPath, "config")); os.IsNotExist(err) {
		cmd := exec.Command(m.BinaryPath, "init")
		cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to initialize IPFS: %v", err)
		}
	}

	m.initialized = true
	return nil
}

func (m *IPFSManager) StartDaemon() error {
	if m.Status == DaemonRunning {
		return nil
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(m.DataPath, 0755); err != nil {
		return fmt.Errorf("failed to create IPFS data directory: %v", err)
	}

	if err := m.Initialize(); err != nil {
		return err
	}

	m.Status = DaemonStarting
	m.Daemon = exec.Command(m.BinaryPath, "daemon")
	m.Daemon.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))

	if err := m.Daemon.Start(); err != nil {
		m.Status = DaemonStopped
		return fmt.Errorf("failed to start IPFS daemon: %v", err)
	}

	// Wait for daemon to start and API to be available
	for i := 0; i < 30; i++ {
		if sh := shell.NewShell("localhost:5001"); sh != nil {
			if _, err := sh.ID(); err == nil {
				m.Shell = sh
				m.Status = DaemonRunning
				return nil
			}
		}
		time.Sleep(time.Second)
	}

	// If we got here, daemon didn't start properly
	m.StopDaemon()
	return fmt.Errorf("timeout waiting for IPFS daemon to start")
}

func (m *IPFSManager) StopDaemon() error {
	if m.Status == DaemonStopped {
		return nil
	}

	m.Status = DaemonStopping

	// Simply set Shell to nil, no need to close it
	m.Shell = nil

	if m.Daemon != nil && m.Daemon.Process != nil {
		if err := m.Daemon.Process.Signal(os.Interrupt); err != nil {
			return fmt.Errorf("failed to send interrupt signal to daemon: %w", err)
		}

		// Wait for process to exit
		if err := m.Daemon.Wait(); err != nil {
			return fmt.Errorf("daemon exit error: %w", err)
		}
	}

	m.Daemon = nil
	m.Status = DaemonStopped
	return nil
}

func (m *IPFSManager) InitializeExistingDaemon() error {
	sh := shell.NewShell("localhost:5001")
	if sh == nil {
		return fmt.Errorf("failed to create shell connection to IPFS daemon")
	}

	// Try to verify the connection by calling ID
	if _, err := sh.ID(); err != nil {
		return fmt.Errorf("no running IPFS daemon found: %w", err)
	}

	// Connection successful, update manager state
	m.Shell = sh
	m.Status = DaemonRunning
	return nil
}

func (m *IPFSManager) IsDaemonRunning() bool {
	if m.Status != DaemonRunning || m.Shell == nil {
		return false
	}

	// Try to ping the API to confirm it's really running
	_, err := m.Shell.ID()
	return err == nil
}

func (m *IPFSManager) GetNodeInfo() (string, []string, error) {
	if !m.IsDaemonRunning() || m.Shell == nil {
		return "", nil, fmt.Errorf("IPFS daemon is not running")
	}

	info, err := m.Shell.ID()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get node info: %v", err)
	}

	return info.ID, info.Addresses, nil
}
