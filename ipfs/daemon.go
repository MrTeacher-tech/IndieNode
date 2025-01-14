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
	if _, err := os.Stat(filepath.Join(m.DataPath, "config")); os.IsNotExist(err) {
		cmd := exec.Command(m.BinaryPath, "init")
		cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to initialize IPFS: %v", err)
		}
	}
	return nil
}

func (m *IPFSManager) StartDaemon() error {
	if err := m.Initialize(); err != nil {
		return err
	}

	m.Daemon = exec.Command(m.BinaryPath, "daemon", "--init")
	m.Daemon.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))
	
	if err := m.Daemon.Start(); err != nil {
		return fmt.Errorf("failed to start IPFS daemon: %v", err)
	}
	
	// Wait for daemon to start
	time.Sleep(3 * time.Second)
	
	// Initialize shell connection
	m.Shell = shell.NewShell("localhost:5001")
	return nil
}

func (m *IPFSManager) StopDaemon() error {
	if m.Daemon != nil && m.Daemon.Process != nil {
		m.Daemon.Process.Signal(os.Interrupt)
		return m.Daemon.Wait()
	}
	return nil
}
