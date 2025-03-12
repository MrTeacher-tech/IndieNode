package ipfs

import (
	"bufio"
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

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(m.DataPath, 0755); err != nil {
		return fmt.Errorf("failed to create IPFS data directory: %v", err)
	}

	// Check if IPFS is already initialized in our custom directory
	configPath := filepath.Join(m.DataPath, "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Initialize IPFS with our custom data directory
		cmd := exec.Command(m.BinaryPath, "init")
		cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to initialize IPFS: %v (output: %s)", err, string(output))
		}

		// Configure the addresses one at a time
		configCommands := []struct {
			args []string
			desc string
		}{
			{[]string{"config", "Addresses.API", "/ip4/127.0.0.1/tcp/5001"}, "API address"},
			{[]string{"config", "Addresses.Swarm", "--json", `["/ip4/0.0.0.0/tcp/4001","/ip4/0.0.0.0/tcp/8081/ws","/ip4/0.0.0.0/udp/4001/quic-v1","/ip4/0.0.0.0/udp/4001/quic-v1/webtransport"]`}, "Swarm addresses"},
			{[]string{"config", "--json", "Routing", `{"Type":"dht","Methods":{"find-peers":{"Strategy":"dht"},"find-providers":{"Strategy":"dht"},"get-ipns":{"Strategy":"dht"},"provide":{"Strategy":"dht"},"put-ipns":{"Strategy":"dht"}}}`}, "DHT routing"},
			{[]string{"config", "--json", "Swarm.EnableHolePunching", "true"}, "Enable hole punching"},
			{[]string{"config", "--json", "Swarm.EnableRelayClient", "true"}, "Enable relay client"},
			{[]string{"config", "--json", "Swarm.RelayClient.EnableCircuitV1", "true"}, "Enable circuit relay v1"},
			{[]string{"config", "--json", "Swarm.RelayService.Enabled", "true"}, "Enable relay service"},
		}

		for _, cmd := range configCommands {
			command := exec.Command(m.BinaryPath, cmd.args...)
			command.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))
			if output, err := command.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to configure IPFS %s: %v (output: %s)", cmd.desc, err, string(output))
			}
			fmt.Printf("Successfully configured %s\n", cmd.desc)
		}
	}

	m.initialized = true
	return nil
}

func (m *IPFSManager) StartDaemon() error {
	if m.Status == DaemonRunning {
		return nil
	}

	// Initialize IPFS if not already done
	if err := m.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize IPFS: %v", err)
	}

	m.Status = DaemonStarting
	fmt.Printf("[DEBUG] Starting IPFS daemon with IPFS_PATH=%s\n", m.DataPath)

	// Clear and reinitialize gateways
	fmt.Printf("[DEBUG] Reinitializing gateways...\n")
	m.ReinitializeGateways()

	// Configure ports:
	// API port (5001) - for sending commands to IPFS
	// Gateway port (8080) - for accessing content
	// Swarm ports (4001/tcp, 8081/ws) - for peer connections
	fmt.Printf("[DEBUG] Configuring IPFS ports...\n")
	configCommands := []struct {
		args []string
		desc string
	}{
		{[]string{"config", "Addresses.API", "/ip4/127.0.0.1/tcp/5001"}, "API address"},
		{[]string{"config", "Addresses.Gateway", "/ip4/0.0.0.0/tcp/8080"}, "Gateway address"},
		{[]string{"config", "Addresses.Swarm", "--json", `["/ip4/0.0.0.0/tcp/4001","/ip4/0.0.0.0/tcp/8081/ws","/ip4/0.0.0.0/udp/4001/quic-v1","/ip4/0.0.0.0/udp/4001/quic-v1/webtransport"]`}, "Swarm addresses"},
		{[]string{"config", "--json", "Gateway", `{"HTTPHeaders":{"Access-Control-Allow-Origin":["*"],"Access-Control-Allow-Methods":["GET"],"Access-Control-Allow-Headers":["X-Requested-With","Range","User-Agent"]}}`}, "Gateway CORS"},
		{[]string{"config", "--json", "Swarm.EnableHolePunching", "true"}, "Enable hole punching"},
		{[]string{"config", "--json", "Swarm.EnableRelayClient", "true"}, "Enable relay client"},
		{[]string{"config", "--json", "Swarm.RelayClient.EnableCircuitV1", "true"}, "Enable circuit relay v1"},
		{[]string{"config", "--json", "Swarm.RelayService.Enabled", "true"}, "Enable relay service"},
	}

	for _, cmd := range configCommands {
		command := exec.Command(m.BinaryPath, cmd.args...)
		command.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))
		if output, err := command.CombinedOutput(); err != nil {
			fmt.Printf("[ERROR] Failed to configure IPFS %s: %v (output: %s)\n", cmd.desc, err, string(output))
		} else {
			fmt.Printf("[DEBUG] Successfully configured %s\n", cmd.desc)
		}
	}

	m.Daemon = exec.Command(m.BinaryPath, "daemon")
	m.Daemon.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))

	// Capture stdout and stderr
	output, err := m.Daemon.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	errOutput, err := m.Daemon.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Start reading output in goroutines
	go func() {
		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			fmt.Printf("IPFS daemon stdout: %s\n", scanner.Text())
		}
	}()
	go func() {
		scanner := bufio.NewScanner(errOutput)
		for scanner.Scan() {
			fmt.Printf("IPFS daemon stderr: %s\n", scanner.Text())
		}
	}()

	if err := m.Daemon.Start(); err != nil {
		m.Status = DaemonStopped
		return fmt.Errorf("failed to start IPFS daemon: %v", err)
	}

	// Wait for daemon to start and API to be available
	fmt.Println("Waiting for IPFS daemon to start...")
	for i := 0; i < 30; i++ {
		if sh := shell.NewShell("localhost:5001"); sh != nil {
			if info, err := sh.ID(); err == nil {
				// Verify we have addresses configured
				if len(info.Addresses) > 0 {
					fmt.Printf("IPFS daemon started successfully. Node ID: %s\n", info.ID)
					fmt.Printf("Addresses: %v\n", info.Addresses)
					m.Shell = sh
					m.Status = DaemonRunning
					return nil
				}
				fmt.Println("IPFS daemon running but no addresses configured")
			} else {
				fmt.Printf("Attempt %d: API not ready: %v\n", i+1, err)
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

func (m *IPFSManager) ConfigureGateway() error {
	cmd := exec.Command(m.BinaryPath, "config", "Addresses.Gateway", "/ip4/127.0.0.1/tcp/8080")
	cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to configure gateway: %v (output: %s)", err, string(output))
	}
	fmt.Printf("Successfully configured gateway address\n")
	return nil
}
