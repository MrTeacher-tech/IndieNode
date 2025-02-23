package ipfs

import (
	"os/exec"
	"sync"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
)

const (
	IPFSVersion          = "v0.22.0"
	IPFSDataDir          = ".ipfs"
	MinCompatibleVersion = "v0.20.0" // Minimum IPFS version we support
)

type DaemonStatus int

const (
	DaemonStopped DaemonStatus = iota
	DaemonStarting
	DaemonRunning
	DaemonStopping
)

type IPFSMode string

const (
	SystemIPFS      IPFSMode = "system"
	AppSpecificIPFS IPFSMode = "app-specific"
)

var DefaultGateways = []string{
	"http://localhost:8080",        // Local gateway
	"https://cloudflare-ipfs.com",  // Cloudflare gateway (most reliable)
	"https://dweb.link",            // Alternative Cloudflare gateway
	"https://gateway.ipfs.io",      // Protocol Labs gateway
	"https://ipfs.io",              // Main IPFS gateway
}

type IPFSManager struct {
	BinaryPath  string
	DataPath    string
	Daemon      *exec.Cmd
	Shell       *shell.Shell
	Status      DaemonStatus
	Mode        IPFSMode // Current active mode
	initialized bool
	gateways    []GatewayStatus
	gatewayLock sync.RWMutex
}

type Config struct {
	CustomBinaryPath string
	CustomDataPath   string
	CustomGateways   []string
}

type GatewayStatus struct {
	URL      string
	Healthy  bool
	LastUsed time.Time
}
