package ipfs

import (
	"os/exec"
	"sync"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
)

const (
	IPFSVersion          = "v0.33.2"
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
	"https://ipfs.io",         // Protocol Labs main gateway (most reliable)
	"https://gateway.ipfs.io", // Protocol Labs alternative gateway
	"https://dweb.link",       // Protocol Labs dweb gateway
	"https://nftstorage.link", // NFT.Storage gateway
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
	CustomGateways   []string
}

type GatewayStatus struct {
	URL      string
	Healthy  bool
	LastUsed time.Time
}
