# Run from the repository root after: go build -o bin/tokenlens .
# Synthetic data only; no ccusage, exchange-rate requests, or personal logs.
Output docs/assets/demo.gif
Output docs/assets/demo.mp4

Set Shell bash
Set FontSize 18
Set Width 1680
Set Height 1100
Set Padding 16
Set Theme "Catppuccin Mocha"
Set Framerate 15
Set TypingSpeed 45ms
Set CursorBlink false
Env NO_COLOR ""
Env TERM "xterm-256color"
Env COLORTERM "truecolor"

# Start on the populated overview so the first frame explains the product.
Hide
Type "./bin/tokenlens --demo --currency EUR --since 20260901 --until 20260930"
Enter
Wait+Screen /synthetic demo rate/
Sleep 500ms
Show
Sleep 3s

# Inspect an exact day, then compare models and token counts.
Right 3
Sleep 1s
Enter
Sleep 2.5s
Type "3"
Sleep 2s
Type "c"
Sleep 1.5s
Type "c"

# Independent agent filter, then the cache breakdown.
Type "a"
Sleep 1.5s
Type "x4"
Sleep 2.5s

# Weekly grouping and a different display currency (synthetic demo rate).
Type "1w"
Sleep 2s
Type "e"
Sleep 2s

# Show the light theme and the export controls.
Ctrl+t
Type "light"
Sleep 1s
Enter
Sleep 2s
Type "o"
Sleep 2s
Escape

# Return to the original dashboard for a clean loop.
Ctrl+t
Type "dark"
Enter
Type "eeed"
Sleep 3s
Hide
Type "q"
