Output demos/demo-malicious.gif

Set Theme "Catppuccin Mocha"
Set FontSize 16
Set Width 1200
Set Height 700
Set Shell "bash"
Set TypingSpeed 80ms
Set Padding 20

# Launch skill-inspector on a skill with hidden content
Type "./skill-inspector ./demos/malicious-skill.md"
Sleep 500ms
Enter
Sleep 1s

# Scroll through the source — it looks innocent
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
Sleep 600ms

# Now reveal hidden content
Tab
Sleep 1.2s

# Pause on frontmatter findings
Sleep 2s

# Scroll down to see HTML comments and suspicious chars
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 800ms

# Pause to let the suspicious characters section sink in
Sleep 2.5s

# Quit
Type "q"
Sleep 500ms
