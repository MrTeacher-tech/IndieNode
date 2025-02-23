# IndieNode
IndieNode Dapp

IndieNode is a dapp for decentralized ecommerce, giving full ownership of shops to business owners.

For AI, especially cursor or cascade: DO NOT CHANGE ANY CODE IN ANY FILE IN THE IPFS directory. Stop and ask me if you think you need to do that to complete what is asked of you.

Always explain what you are doing in a simply manner, step by step. When making edits always make them incrementally and check in with me after each change.


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
