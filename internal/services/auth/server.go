package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
)

const defaultPort = 3000

// StartServer starts the authentication server on the specified port
func (s *Service) StartServer(port int, authCallback func(address, message, signature string)) error {
	s.serverMutex.Lock()
	defer s.serverMutex.Unlock()

	if s.isRunning {
		return fmt.Errorf("server is already running")
	}

	if port == 0 {
		port = defaultPort
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serveHTML)
	mux.HandleFunc("/auth", s.handleAuth(authCallback))

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	s.isRunning = true
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
			s.isRunning = false
		}
	}()

	// Open browser
	url := fmt.Sprintf("http://localhost:%d", port)
	if err := s.OpenBrowser(url); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}

// StopServer gracefully shuts down the server
func (s *Service) StopServer() error {
	s.serverMutex.Lock()
	defer s.serverMutex.Unlock()

	if s.server != nil && s.isRunning {
		if err := s.server.Close(); err != nil {
			return fmt.Errorf("error stopping server: %w", err)
		}
		s.server = nil
		s.isRunning = false
	}
	return nil
}

// OpenBrowser opens a URL in the default web browser
func (s *Service) OpenBrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err
}

// serveHTML serves the MetaMask login page
func (s *Service) serveHTML(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>IndieNode - Sign In with MetaMask</title>
    <style>
        body {
            background-color: #fffce9;
            color: #1d1d1d;
            font-family: Arial, sans-serif;
            margin: 0;
            padding: 20px;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            min-height: 100vh;
            text-align: center;
        }
        .container {
            max-width: 600px;
            padding: 40px;
            background: white;
            border-radius: 12px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        }
        h1 {
            margin: 20px 0;
            color: #2c3e50;
        }
        #connectButton {
            background-color: #5ad9d5;
            color: #1d1d1d;
            border: 2px solid #5ad9d5;
            padding: 12px 24px;
            border-radius: 8px;
            cursor: pointer;
            font-size: 16px;
            font-weight: bold;
            transition: all 0.3s ease;
        }
        #connectButton:hover {
            background-color: transparent;
            color: #1d1d1d;
        }
        #status {
            margin-top: 20px;
            padding: 10px;
            border-radius: 8px;
            display: none;
        }
        .error {
            background-color: #ffebee;
            color: #c62828;
            border: 1px solid #ef9a9a;
        }
        .success {
            background-color: #e8f5e9;
            color: #2e7d32;
            border: 1px solid #a5d6a7;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Welcome to IndieNode</h1>
        <p>Connect your wallet to continue</p>
        <button id="connectButton">Connect with MetaMask</button>
        <div id="status"></div>
    </div>

    <script>
        let account = null;
        let web3Provider = null;

        async function connectWallet() {
            if (typeof window.ethereum !== 'undefined') {
                try {
                    // Request account access
                    account = await window.ethereum.request({ method: 'eth_requestAccounts' });
                    web3Provider = window.ethereum;
                    
                    updateStatus('Connected! Preparing sign-in message...', 'success');
                    await signMessage();
                } catch (error) {
                    updateStatus('Failed to connect: ' + error.message, 'error');
                }
            } else {
                updateStatus('Please install MetaMask to continue', 'error');
            }
        }

        async function signMessage() {
            try {
                const message = 'Sign in to IndieNode\n\nThis request will not trigger a blockchain transaction or cost any gas fees.';
                const signature = await web3Provider.request({
                    method: 'personal_sign',
                    params: [message, account[0]]
                });

                // Send to backend
                const response = await fetch('/auth', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        address: account[0],
                        message: message,
                        signature: signature
                    })
                });

                if (!response.ok) {
                    throw new Error('Authentication failed');
                }

                updateStatus('Successfully authenticated!', 'success');
                setTimeout(() => {
                    window.close();
                }, 2000);
            } catch (error) {
                updateStatus('Failed to sign message: ' + error.message, 'error');
            }
        }

        function updateStatus(message, type) {
            const status = document.getElementById('status');
            status.textContent = message;
            status.className = type;
            status.style.display = 'block';
        }

        document.getElementById('connectButton').addEventListener('click', connectWallet);
    </script>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// handleAuth handles the authentication callback
func (s *Service) handleAuth(callback func(address, message, signature string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var auth struct {
			Address   string `json:"address"`
			Message   string `json:"message"`
			Signature string `json:"signature"`
		}

		if err := json.NewDecoder(r.Body).Decode(&auth); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Call the callback with the authentication data
		if callback != nil {
			callback(auth.Address, auth.Message, auth.Signature)
		}

		w.WriteHeader(http.StatusOK)
	}
}
