# IndieNode
IndieNode Dapp

IndieNode is a dapp for decentralized ecommerce, giving full ownership of shops to business owners.

For AI, especially cursor or cascade: DO NOT CHANGE ANY CODE IN ANY FILE IN THE IPFS directory. Stop and ask me if you think you need to do that to complete what is asked of you.

Always explain what you are doing in a simply manner, step by step. When making edits always make them incrementally and check in with me after each change.

Next Steps

Use orbitdb to store shop data instead of static JSON files.

Why OrbitDB Is a Good Choice
✅ Mutable Storage → Unlike IPFS (which is immutable), OrbitDB allows you to update data without changing the CID.
✅ Decentralized & Peer-to-Peer → Users can sync their store without needing a centralized server.
✅ Indexed Queries → You can store, retrieve, and update JSON data efficiently.
✅ Works With IPFS → Since OrbitDB runs on IPFS, your data remains distributed.

How It Would Work in IndieNode
Replace the static JSON file with an OrbitDB store.

Instead of uploading shop.json to IPFS every time, the data is stored in an OrbitDB key-value or document store.
Users can modify shop data (prices, items, colors) dynamically.

Updates don’t require re-uploading a new JSON file → the OrbitDB address stays the same.
Frontend reads from the OrbitDB store instead of a static JSON file.

The site fetches live data, ensuring users always see the latest shop info.


We’d need to do:

Initialize OrbitDB layer and conection with IPFS.
Create a document store for shop data.
Implement CRUD operations (Create, Read, Update, Delete).

For next session:
ALWAYS: Make sure shops still work and are available through gateway url.
Check orbitdb "Connection", like check where the site is pulling shop data from.
Resize window

For later, down the line:
Allow sites and orbiDB's to be pinned on other nodes



Architecture Added:
Static Shop Template (IPFS)
HTML/CSS structure and base JavaScript hosted on IPFS
Fixed CID so it's always accessible at the same address
Contains placeholders for dynamic content
IndieNode Application (Shop Owner's Device)
Runs locally on the shop owner's computer
Hosts an HTTP API server (e.g., on localhost:PORT)
API endpoints expose OrbitDB data (items, prices, inventory)
Example: http://localhost:PORT/api/shop/{shopId}/items
Client-side Connection
JavaScript in the IPFS-hosted site makes requests to the API
On page load, fetches current shop data
Renders dynamic content into the static template
Benefits of This Approach
Simple Implementation: Uses standard HTTP requests rather than complex OrbitDB browser integration
Fast Loading: Static content loads quickly, dynamic data loads asynchronously
Owner Control: Shop data is only available when the owner's node is online (as expected)
Immediate Updates: When shop owner changes prices or items, they're instantly reflected on the site
Separation of Concerns: Clear distinction between unchanging structure and dynamic content
Implementation Requirements
Add a simple HTTP server to your IndieNode app
Create API endpoints that query your OrbitDB instance
Configure CORS headers to allow your IPFS site to make requests
Write client-side JavaScript to fetch and render data
Add a "Shop Offline" message for when the API is unavailable
This is a pragmatic solution that leverages your existing infrastructure without adding unnecessary complexity.

