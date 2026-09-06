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

# Start on a populated overview; all usage and exchange rates are synthetic.
Hide
Type "./bin/tokenlens --demo --no-cache --timezone UTC --currency EUR --since 20260901 --until 20260930"
Enter
Wait+Screen /synthetic demo rate/
Sleep 500ms
Show
Sleep 3.5s

# Compare models and understand the token/cache mix.
Type "3"
Sleep 3s
Type "4"
Sleep 3s
Type "1w"
Sleep 2.5s

# Show the whole picker, then fuzzy-search and apply Tokyo Night Dark.
Ctrl+t
Sleep 2s
Type "tnd"
Sleep 2s
Enter
Sleep 1.5s

# Preview and apply Solarized Dark.
Ctrl+t
Type "solarized dark"
Sleep 2s
Enter
Sleep 2s

# Restore the starting palette and daily view for the loop.
Ctrl+t
Type "dark"
Enter
Type "d"
Sleep 2.5s
Hide
Type "q"
