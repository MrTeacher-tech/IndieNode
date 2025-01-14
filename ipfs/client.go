package ipfs

import (
	"fmt"
	"os"
)

func (m *IPFSManager) AddFile(path string) (string, error) {
	if m.Shell == nil {
		return "", fmt.Errorf("IPFS shell not initialized")
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash, err := m.Shell.Add(file)
	if err != nil {
		return "", err
	}

	return hash, nil
}

func (m *IPFSManager) AddDirectory(path string) (string, error) {
	if m.Shell == nil {
		return "", fmt.Errorf("IPFS shell not initialized")
	}

	hash, err := m.Shell.AddDir(path)
	if err != nil {
		return "", err
	}

	return hash, nil
}

func (m *IPFSManager) GetGatewayURL(hash string) string {
	return fmt.Sprintf("https://ipfs.io/ipfs/%s", hash)
}
