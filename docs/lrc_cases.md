The formatting issues are still there, in the ui side too many paths:

### webui:

- commit
- commit and push
- vouch (only during review)
- skip (only during review)
- abort

### terminal:

- commit
- vouch ctrl-v (only during review)
- skip ctrl-s (only during review)
- abort ctrl-c

### Other things which affect all stages:

- triggered with "git commit -m 'message'"
- triggered with "git commit" (no message); triggers "editor" from terminal; doesn't let commmit without message filled from webui
- triggered with "lrc review --force" -- everything works exactly as other case

### other things to take care of

- doesn't mess with formatting, later typing in information in the terminal, etc. no weird indentations, etc
- works in windows, linux, macos
- gives error message and blocks commit in vscode if review or skip or vouch not already done
- ctrl-c always works from terminal for abort
- Enter works to continue commit; if message already provided - show it and just commit; if not provided open editor.

