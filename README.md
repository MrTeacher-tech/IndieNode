# IndieNode
IndieNode Dapp

IndieNode is a dapp for decentralized ecommerce, giving full ownership of shops to business owners.

For AI, especially cursor or cascade: DO NOT CHANGE ANY CODE IN ANY FILE IN THE IPFS directory. Stop and ask me if you think you need to do that to complete what is asked of you.

Always explain what you are doing in a simply manner, step by step. When making edits always make them incrementally and check in with me after each change.

### IPFS Gateway Configuration
The IPFS gateway (127.0.0.1:8080) is configured to remain stable even after IndieNode exits. This ensures published shops stay accessible without requiring constant management. Do not manually clear or modify the gateway configuration unless you specifically intend to unpublish all shops.

When we were having gateway issues, it was very helpful to run IPFS CLI commands like this:

'''
(base) andrewverrilli@Andrews-MBP ~ % ~/indie_node_ipfs/ipfs ls QmfZ5vZWDwCwpW34eMwqy7G9ccdssMWvbrxomEH69FkGRh

QmW8h5naDxGqwacppUgEe3dAPhGWqjbm4VFp4YwvVHv2oZ - Dev Test Shop/
(base) andrewverrilli@Andrews-MBP ~ % ls -R ~/Desktop/Dapps/IndieNode/shops/Dev\ Test\ Shop/
ipfs_metadata.json	shop.json		src

/Users/andrewverrilli/Desktop/Dapps/IndieNode/shops/Dev Test Shop//src:
assets		index.html	items		styles.css	web3.js

/Users/andrewverrilli/Desktop/Dapps/IndieNode/shops/Dev Test Shop//src/assets:
logos

/Users/andrewverrilli/Desktop/Dapps/IndieNode/shops/Dev Test Shop//src/assets/logos:

/Users/andrewverrilli/Desktop/Dapps/IndieNode/shops/Dev Test Shop//src/items:
dev1.jpeg	dev2.jpeg	dev3.jpeg	dev4.jpeg	dev5.jpeg	dev6.jpeg	dev7.jpeg	dev8.jpeg
(base) andrewverrilli@Andrews-MBP ~ % 
'''

Here is a common error I often get:

Error: "container.NewTabItem undefined (type *fyne.Container has no field or method NewTabItem)"
Cause: Variable shadowing - a local variable named 'container' is hiding the 'container' package name
Solution: Rename local variable 'container' to 'containerObj' or similar to prevent shadowing the package name
Example Fix:
- Bad:  container := obj.(*fyne.Container)
- Good: containerObj := obj.(*fyne.Container)


Error: "no new variables on left side of :=" when using = instead of := with container.NewTabItemWithIcon()

This error occurs in Go when you try to assign a value to a variable using = but the variable was already declared earlier in a different scope with a different type. The solution is to:

1. Use a new variable name with := for the new declaration
2. Update all subsequent references to use the new variable name

Example fix:
// Before (error)
tabItem = container.NewTabItemWithIcon(...)

// After (fixed)
newTabItem := container.NewTabItemWithIcon(...)

Notes from yesterday:
What we fixed today:

Gateway URLs - Fixed the double path issue (/ipfs/cid/src/index.html/src/index.html)
Port configuration - Ensured correct ports (5001 for API, 8080 for gateway)
URL construction - Added proper checks to avoid path duplication
Still needs fixing:

CSS/JS not loading issue
Likely cause: IPFS directory structure handling in AddDirectory() function
Potential fix: Add --wrap-with-directory flag to ipfs add command or check how directory structure is being preserved

Remember: If you want to use IPFS commands you have to use ~/indie_node_ipfs/ipfs, that is where the binary is located when we install ipfs using indienode

Next steps:

When I publish a shop, (for example my dev-test-shop), what shows up in the terminal is: 
'''
Updated shop.json with new CID
Final URL: https://ipfs.io/ipfs/QmecbyjeVQ6e18p7Jy9xdYMt99nLvXs4fvbFXSaiFQrLCa/src/index.html

But the real gateay url that is publicly accesible is this:
https://ipfs.io/ipfs/QmfZ5vZWDwCwpW34eMwqy7G9ccdssMWvbrxomEH69FkGRh/Dev%20Test%20Shop/src/index.html
'''

So I need the terminal and the view shop tab that shows the gateways to provide the correctt gateway url. The url with the shop directory name in the title is the one that works.

Two main files need to be checked:

/Users/andrewverrilli/Desktop/Dapps/IndieNode/ipfs/client.go - handles URL construction in the Publish function
/Users/andrewverrilli/Desktop/Dapps/IndieNode/internal/ui/windows/main_window.go - handles displaying the gateway URL in the UI


