# Allow command list
command(git)                       # Standard git commands
command(npm run (build|lint|test)) # Allow safe npm scripts via regex
command(npm install)
command((go|wails) .*)
command((Get-ChildItem|mkdir|dir|Get-Item|New-Item))
unsandboxed(git push)              # Allow git push outside sandbox
read_file(/var/log/app)            # Read external log paths
write_file(src/)                   # Edit relative src/ folder
read_url(google.com)               # Fetch Google subdomains
mcp(linter/*)                      # Run linter MCP tools

# Deny command list
command(rm -rf)                    # Block destructive deletions
command(curl .*)                   # Block unvetted curl downloads
command(sudo)                      # Block sudo privileges
write_file(.git/)                  # Safeguard Git history
write_file(/home/user/.ssh)        # Safeguard SSH keys