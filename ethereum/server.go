package ethereum

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

var server *http.Server

// OpenBrowser opens a URL in the default web browser
func OpenBrowser(url string) error {
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

// ServeHTML serves the MetaMask login page
func ServeHTML(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>MetaMask Login</title>
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
        .logo {
            width: 200px;
            margin-bottom: 20px;
        }
        h1 {
            margin: 20px 0;
        }
        #connectButton {
            background-color: #5ad9d5;
            color: #1d1d1d;
            border: 2px solid #5ad9d5;
            padding: 10px 20px;
            border-radius: 5px;
            cursor: pointer;
            font-size: 16px;
            transition: all 0.3s ease;
        }
        #connectButton:hover {
            background-color: transparent;
            color: #1d1d1d;
        }
        #status {
            margin-top: 20px;
            padding: 10px;
            border: 2px solid #5ad9d5;
            border-radius: 5px;
            display: none;
        }
    </style>
</head>
<body>
    <img src="/assets/logo.png" alt="IndieNode Logo" class="logo">
    <h1>Login with MetaMask</h1>
    <button id="connectButton">Connect Wallet</button>
    <p id="status"></p>

    <script>
        // Show status element when it has content
        const statusElement = document.getElementById('status');
        const originalSetText = statusElement.textContent;
        Object.defineProperty(statusElement, 'textContent', {
            set: function(value) {
                originalSetText.call(this, value);
                this.style.display = value ? 'block' : 'none';
            }
        });

        const connectButton = document.getElementById('connectButton');
        const statusText = document.getElementById('status');

        connectButton.addEventListener('click', async () => {
            if (window.ethereum) {
                try {
                    const accounts = await window.ethereum.request({ method: 'eth_requestAccounts' });
                    const account = accounts[0];
                    statusText.textContent = 'Connected: ' + account;
                    console.log('Connected to MetaMask account: ' + account);
                    
                    // Create a message to sign
                    const message = "Sign this message to authenticate with IndieNode";
                    
                    // Request signature
                    const signature = await window.ethereum.request({
                        method: 'personal_sign',
                        params: [message, account]
                    });
                    
                    // Send the address and signature to our backend
                    const response = await fetch('/authenticate', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                        },
                        body: JSON.stringify({
                            address: account,
                            message: message,
                            signature: signature
                        })
                    });
                    
                    if (response.ok) {
                        statusText.textContent = 'Authentication successful! You can close this window.';
                        // Close the window after a brief delay
                        setTimeout(() => window.close(), 2000);
                    } else {
                        throw new Error('Authentication failed');
                    }
                } catch (error) {
                    console.error('Error connecting to MetaMask:', error);
                    statusText.textContent = 'Error connecting to MetaMask. Check the console for details.';
                }
            } else {
                statusText.textContent = 'MetaMask not installed. Please install MetaMask and try again.';
                console.error('MetaMask not detected.');
            }
        });
    </script>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// ConnectToMetaMask starts the server and opens the browser for MetaMask connection
func ConnectToMetaMask(authCallback func(address, message, signature string)) {
    fmt.Println("Starting MetaMask connection server at http://localhost:8080")
    
    // Initialize server if not already running
    if server == nil {
        mux := http.NewServeMux()
        mux.HandleFunc("/", ServeHTML)
        
        // Serve logo file from IndieNode_assets directory
        mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
            if r.URL.Path == "/assets/logo.png" {
                http.ServeFile(w, r, "IndieNode_assets/indieNode_logo.png")
                return
            }
            http.NotFound(w, r)
        })
        
        mux.HandleFunc("/authenticate", func(w http.ResponseWriter, r *http.Request) {
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

            // Call the provided authentication callback
            authCallback(auth.Address, auth.Message, auth.Signature)

            w.WriteHeader(http.StatusOK)
        })
        
        server = &http.Server{
            Addr:    ":8080",
            Handler: mux,
        }
        
        // Start server in a goroutine
        go func() {
            if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                fmt.Printf("Error starting server: %v\n", err)
            }
        }()
        
        // Open browser
        time.Sleep(100 * time.Millisecond) // Give server time to start
        if err := OpenBrowser("http://localhost:8080"); err != nil {
            fmt.Printf("Error opening browser: %v\n", err)
        }
    }
}
