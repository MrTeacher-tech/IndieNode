/**
 * ShopAPI - Client-side library for accessing shop data from the IndieNode API
 */
class ShopAPI {
    /**
     * Initialize the ShopAPI
     * 
     * @param {Object} options - Configuration options
     * @param {string} options.apiUrl - Base URL for the API (default: http://localhost:8000)
     * @param {string} options.shopId - ID of the shop to load
     * @param {function} options.onLoad - Callback when shop data is loaded
     * @param {function} options.onError - Callback when an error occurs
     */
    constructor(options = {}) {
        this.apiUrl = options.apiUrl || 'http://localhost:8000';
        this.shopId = options.shopId;
        this.onLoad = options.onLoad || (() => {});
        this.onError = options.onError || ((err) => console.error('Shop API Error:', err));
        
        this.shop = null;
        this.items = [];
        this.isConnected = false;
        
        // Verify the API is running
        this.checkConnection();
    }
    
    /**
     * Check if the API is running
     */
    async checkConnection() {
        try {
            const response = await fetch(`${this.apiUrl}/api`);
            if (!response.ok) throw new Error('API returned an error');
            
            const data = await response.json();
            if (data.success) {
                console.log('Connected to IndieNode API:', data.data.status);
                this.isConnected = true;
                
                // If shop ID is provided, load the shop data
                if (this.shopId) {
                    await this.loadShop();
                }
            } else {
                throw new Error(data.error || 'Unknown API error');
            }
        } catch (err) {
            console.error('Failed to connect to IndieNode API:', err);
            this.isConnected = false;
            this.onError(err);
        }
    }
    
    /**
     * Load shop data for the specified shop ID
     * 
     * @param {string} shopId - Override the shop ID provided in constructor
     */
    async loadShop(shopId) {
        if (shopId) this.shopId = shopId;
        if (!this.shopId) {
            throw new Error('No shop ID provided');
        }
        
        try {
            const response = await fetch(`${this.apiUrl}/api/shops/${this.shopId}`);
            if (!response.ok) throw new Error('API returned an error');
            
            const data = await response.json();
            if (data.success) {
                this.shop = data.data;
                console.log('Loaded shop data:', this.shop.Name);
                this.onLoad(this.shop);
                return this.shop;
            } else {
                throw new Error(data.error || 'Failed to load shop data');
            }
        } catch (err) {
            console.error('Error loading shop data:', err);
            this.onError(err);
            return null;
        }
    }
    
    /**
     * Load shop items separately
     */
    async loadItems() {
        if (!this.shopId) {
            throw new Error('No shop ID provided');
        }
        
        try {
            const response = await fetch(`${this.apiUrl}/api/shops/${this.shopId}/items`);
            if (!response.ok) throw new Error('API returned an error');
            
            const data = await response.json();
            if (data.success) {
                this.items = data.data;
                console.log(`Loaded ${this.items.length} items`);
                return this.items;
            } else {
                throw new Error(data.error || 'Failed to load shop items');
            }
        } catch (err) {
            console.error('Error loading shop items:', err);
            this.onError(err);
            return [];
        }
    }
    
    /**
     * Render items to a container element
     * 
     * @param {HTMLElement} container - Container element to render items into
     */
    renderItems(container) {
        if (!container) {
            console.error('No container provided for renderItems');
            return;
        }
        
        // Clear the container
        container.innerHTML = '';
        
        if (!this.items || this.items.length === 0) {
            container.innerHTML = '<p class="empty-items">No items available</p>';
            return;
        }
        
        // Create and append item elements
        this.items.forEach(item => {
            const itemElement = document.createElement('div');
            itemElement.className = 'item-card';
            
            const imagesHtml = item.PhotoPaths && item.PhotoPaths.length > 0 
                ? `<div class="item-images">
                    ${item.PhotoPaths.map(path => 
                        `<img src="../${path}" class="item-image" alt="Product Image" 
                         onclick="openModal(this.src)">`
                    ).join('')}
                   </div>`
                : '';
            
            itemElement.innerHTML = `
                ${imagesHtml}
                <div class="item-info">
                    <div class="item-name">${item.Name}</div>
                    <div class="item-price">$${item.Price.toFixed(2)}</div>
                    <div class="item-description">${item.Description}</div>
                </div>
                <button class="eth-buy-button" data-item-id="${item.ID}" data-item-price="${item.Price}">
                    Buy with ETH
                </button>
            `;
            
            container.appendChild(itemElement);
        });
        
        // Initialize buy buttons if web3 is available
        if (window.initializeBuyButtons) {
            window.initializeBuyButtons();
        }
    }
    
    /**
     * Handle shop offline state
     * 
     * @param {HTMLElement} container - Container to show offline message
     */
    showOfflineMessage(container) {
        if (!container) return;
        
        container.innerHTML = `
            <div class="shop-offline">
                <h2>Shop is currently offline</h2>
                <p>The shop owner's node is currently not available. Please try again later.</p>
            </div>
        `;
    }
} 