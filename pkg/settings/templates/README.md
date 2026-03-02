# GitSense Chat CLI Templates Configuration

This directory contains the launch templates for editors and terminals used by the `gsc contract launch` command.

## File Structure

The templates are separated by operating system to ensure compatibility:

- `templates.darwin.json`: Configuration for macOS.
- `templates.linux.json`: Configuration for Linux distributions.
- `templates.windows.json`: Configuration for Windows.

The CLI automatically loads the file matching your current operating system.

## How to Customize

1. Open the JSON file corresponding to your OS.
2. Edit the `editors` or `terminals` objects.
3. Save the file.
4. Restart your GitSense Chat session or CLI to apply changes.

## Template Format

Templates use a simple key-value pair where the key is the alias (used in `gsc contract create --editor <alias>`) and the value is the command string.

The `%s` placeholder in the command string will be replaced with the target file path or directory.

### Example: Adding a Custom Terminal

If you want to launch iTerm2 with Vim automatically on macOS, you can add a custom entry to `templates.darwin.json`:

```json
{
  "terminals": {
    "iterm2": "open -a iTerm %s",
    "iterm2-vim": "osascript -e 'tell application \"iTerm\" to create window with default profile command \"cd %s && vim\"'"
  }
}
```

You can then create a contract using this new alias:

```bash
gsc contract create --code 123456 --description "Vim Workflow" --terminal iterm2-vim
```

## Using Scripts

You can also point a template to a custom script on your system. This allows for complex logic (like changing iTerm profiles based on the directory) without cluttering the JSON file.

Example:
```json
{
  "terminals": {
    "my-script": "/Users/yourname/scripts/launch.sh %s"
  }
}
```

## Notes

- Ensure the command you provide is available in your system's PATH.
- On macOS, use `open -a <ApplicationName>` for GUI apps.
- On Linux, use your specific terminal command (e.g., `gnome-terminal`, `xterm`).
- On Windows, use `start` or the full path to the executable.
