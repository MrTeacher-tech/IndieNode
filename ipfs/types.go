package ipfs

import (
	"os/exec"
	shell "github.com/ipfs/go-ipfs-api"
)

const (
	IPFSVersion = "v0.22.0"
	IPFSDataDir = ".ipfs"
)

type IPFSManager struct {
	BinaryPath string
	DataPath   string
	Daemon     *exec.Cmd
	Shell      *shell.Shell
}

type Config struct {
	CustomBinaryPath string
	CustomDataPath   string
}
