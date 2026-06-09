Output demos/demo-clean.gif

Set Theme "Catppuccin Mocha"
Set FontSize 16
Set Width 1200
Set Height 700
Set Shell "bash"
Set TypingSpeed 80ms
Set Padding 20

# Launch skill-inspector on a clean skill file
Type "./skill-inspector ./demos/clean-skill.md"
Sleep 500ms
Enter
Sleep 1s

# Scroll through the source view
Sleep 800ms
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 400ms
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 600ms

# Page up to go back to top
Type "b"
Sleep 600ms

# Toggle to Hidden Content view
Tab
Sleep 1s

# Pause to show the all-clear results
Sleep 2s

# Toggle back to source
Tab
Sleep 800ms

# Quit
Type "q"
Sleep 500ms
