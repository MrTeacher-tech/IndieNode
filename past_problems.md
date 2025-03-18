Fix: Ensure Proper CID Announcement in IPFS DHT
Problem
The app was successfully adding content to IPFS, but the CID wasn’t properly announced in the Distributed Hash Table (DHT). This meant that public IPFS gateways didn’t know where to fetch the content, leading to 504 Gateway Timeout errors.

Solution
After adding a directory to IPFS, the app must:

Announce the CID to the DHT to ensure discoverability:
sh
Copy
Edit
ipfs routing provide <CID>
Verify that the CID is being indexed before returning the gateway URL:
sh
Copy
Edit
ipfs routing findprovs <CID>
If no peers are listed, re-run ipfs routing provide <CID>.
Wait for propagation before assuming the CID is publicly accessible.
Implementation Fix
Modify the app’s IPFS publishing process to include:

Automatic CID announcement (routing provide) after adding the site.
A check (routing findprovs) before returning the gateway URL to confirm the CID is reachable.
A retry mechanism if the CID isn't found, ensuring it gets properly distributed.
This will make sure every published site is immediately accessible on public IPFS gateways without manual intervention.


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
