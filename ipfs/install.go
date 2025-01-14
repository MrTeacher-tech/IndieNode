package ipfs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func NewIPFSManager(config *Config) (*IPFSManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	
	basePath := filepath.Join(homeDir, "indie_node_ipfs")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}
	
	manager := &IPFSManager{
		BinaryPath: filepath.Join(basePath, "ipfs"),
		DataPath:   filepath.Join(basePath, IPFSDataDir),
	}
	
	if runtime.GOOS == "windows" {
		manager.BinaryPath += ".exe"
	}
	
	if config != nil {
		if config.CustomBinaryPath != "" {
			manager.BinaryPath = config.CustomBinaryPath
		}
		if config.CustomDataPath != "" {
			manager.DataPath = config.CustomDataPath
		}
	}
	
	return manager, nil
}

func (m *IPFSManager) EnsureInstalled() error {
	if _, err := os.Stat(m.BinaryPath); os.IsNotExist(err) {
		return m.downloadIPFS()
	}
	return nil
}

func (m *IPFSManager) downloadIPFS() error {
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		return fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
	
	var os string
	switch runtime.GOOS {
	case "darwin":
		os = "darwin"
	case "linux":
		os = "linux"
	case "windows":
		os = "windows"
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	
	url := fmt.Sprintf("https://dist.ipfs.tech/kubo/%s/kubo_%s_%s-%s.tar.gz",
		IPFSVersion, IPFSVersion[1:], os, arch)
	
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	tmpFile, err := os.CreateTemp("", "ipfs-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return err
	}
	
	cmd := exec.Command("tar", "-xzf", tmpFile.Name(), "-C", filepath.Dir(m.BinaryPath))
	if err := cmd.Run(); err != nil {
		return err
	}
	
	extractedPath := filepath.Join(filepath.Dir(m.BinaryPath), "kubo", "ipfs")
	if err := os.Rename(extractedPath, m.BinaryPath); err != nil {
		return err
	}
	
	return os.Chmod(m.BinaryPath, 0755)
}
