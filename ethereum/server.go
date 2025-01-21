package ethereum

import (
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
</head>
<body>
    <h1>Login with MetaMask</h1>
    <button id="connectButton">Connect Wallet</button>
    <p id="status"></p>

    <script>
        const connectButton = document.getElementById('connectButton');
        const statusText = document.getElementById('status');

        connectButton.addEventListener('click', async () => {
            if (window.ethereum) {
                try {
                    const accounts = await window.ethereum.request({ method: 'eth_requestAccounts' });
                    const account = accounts[0];
                    statusText.textContent = 'Connected: ' + account;
                    console.log('Connected to MetaMask account: ' + account);
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
func ConnectToMetaMask() {
	fmt.Println("Starting MetaMask connection server at http://localhost:8080")

	// Initialize server if not already running
	if server == nil {
		mux := http.NewServeMux()
		mux.HandleFunc("/", ServeHTML)

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
