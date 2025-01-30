// Web3 functionality for IndieNode shops

// Global variables
let web3;
let userAccount;
const ETH_USD_PRICE_API = 'https://api.coingecko.com/api/v3/simple/price?ids=ethereum&vs_currencies=usd';

// Wait for the page to fully load
window.addEventListener('load', async () => {
    console.log('🚀 Page loaded, checking MetaMask status...');
    console.log('window.ethereum:', window.ethereum);
    console.log('window.web3:', window.web3);
    
    // Add a small delay to ensure MetaMask has time to inject
    setTimeout(async () => {
        try {
            console.log('Checking providers after delay...');
            console.log('window.ethereum:', window.ethereum);
            console.log('window.ethereum?.isMetaMask:', window.ethereum?.isMetaMask);
            console.log('window.web3:', window.web3);
            
            // Modern dapp browsers
            if (window.ethereum) {
                console.log('🦊 Modern MetaMask provider detected');
                web3 = new Web3(window.ethereum);
                
                // Check if we're already connected
                const accounts = await web3.eth.getAccounts();
                console.log('Found accounts:', accounts);
                
                if (accounts.length > 0) {
                    userAccount = accounts[0];
                    updateConnectedState(true);
                } else {
                    updateConnectedState(false);
                }
                
                // Listen for account changes
                window.ethereum.on('accountsChanged', handleAccountsChanged);
                window.ethereum.on('chainChanged', () => window.location.reload());
                window.ethereum.on('connect', () => console.log('MetaMask Connected'));
                window.ethereum.on('disconnect', () => {
                    console.log('MetaMask Disconnected');
                    handleAccountsChanged([]);
                });
                
            } else if (window.web3) {
                // Legacy dapp browsers
                console.log('🦊 Legacy MetaMask provider detected');
                web3 = new Web3(window.web3.currentProvider);
                const accounts = await web3.eth.getAccounts();
                if (accounts.length > 0) {
                    userAccount = accounts[0];
                    updateConnectedState(true);
                } else {
                    updateConnectedState(false);
                }
            } else {
                console.log('⚠️ No MetaMask provider detected');
                updateConnectedState(false);
            }
            
            // Add click handlers to buttons
            setupBuyButtons();
            
        } catch (error) {
            console.error('Error initializing Web3:', error);
            updateConnectedState(false);
        }
    }, 100); // 100ms delay
});

// Update UI to reflect connection state
function updateConnectedState(isConnected) {
    const buyButtons = document.querySelectorAll('.eth-buy-button');
    buyButtons.forEach(button => {
        if (!web3) {
            button.textContent = 'Install MetaMask';
            button.disabled = true;
            return;
        }
        
        if (isConnected && userAccount) {
            button.textContent = 'Buy with MetaMask';
            button.disabled = false;
        } else {
            button.textContent = 'Connect Wallet to Buy';
            button.disabled = false;
        }
    });
}

// Handle MetaMask account changes
function handleAccountsChanged(accounts) {
    if (accounts.length === 0) {
        userAccount = null;
        updateConnectedState(false);
    } else {
        userAccount = accounts[0];
        updateConnectedState(true);
    }
}

// Connect to MetaMask
async function connectWallet() {
    try {
        const accounts = await window.ethereum.request({ 
            method: 'eth_requestAccounts' 
        });
        handleAccountsChanged(accounts);
        return true;
    } catch (error) {
        console.error('Error connecting to MetaMask:', error);
        if (error.code === 4001) {
            alert('Please connect your MetaMask wallet to make purchases.');
        } else {
            alert('Error connecting to MetaMask. Please try again.');
        }
        return false;
    }
}

// Setup buy button click handlers
function setupBuyButtons() {
    const buyButtons = document.querySelectorAll('.eth-buy-button');
    buyButtons.forEach(button => {
        button.addEventListener('click', async () => {
            if (!web3) {
                window.open('https://metamask.io', '_blank');
                return;
            }
            
            if (!userAccount) {
                const connected = await connectWallet();
                if (!connected) return;
            }
            
            try {
                await prepareTransaction(button);
            } catch (error) {
                console.error('Error preparing transaction:', error);
                alert('Error preparing transaction. Please try again.');
            }
        });
    });
}

// Get current ETH price
async function getEthPrice() {
    try {
        const response = await fetch(ETH_USD_PRICE_API);
        const data = await response.json();
        return data.ethereum.usd;
    } catch (error) {
        console.error('Error fetching ETH price:', error);
        throw new Error('Failed to get ETH price');
    }
}

// Convert USD to ETH
async function convertUSDToETH(usdAmount) {
    const ethPrice = await getEthPrice();
    return usdAmount / ethPrice;
}

// Prepare transaction for an item
async function prepareTransaction(button) {
    try {
        const itemId = button.dataset.itemId;
        const priceUSD = parseFloat(button.dataset.itemPrice);
        const priceETH = await convertUSDToETH(priceUSD);
        const formattedPriceETH = priceETH.toFixed(6);
        const priceWei = web3.utils.toWei(formattedPriceETH, 'ether');
        
        if (confirm(`Confirm purchase for $${priceUSD} (${formattedPriceETH} ETH)?`)) {
            // Transaction sending will be implemented in the next phase
            console.log('Transaction confirmed:', {
                itemId,
                priceUSD,
                priceETH: formattedPriceETH,
                priceWei
            });
        }
    } catch (error) {
        console.error('Error preparing transaction:', error);
        throw new Error('Failed to prepare transaction');
    }
}
