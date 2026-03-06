<!--
Component: GitSense Templates Documentation
Block-UUID: 6f97fb2f-c7e8-4d9f-aadf-07f785e3a6f9
Parent-UUID: 9070
Version: 1.1.0
Description: Updated documentation to reflect the new 'commands/' subdirectory structure and simplified filenames (e.g., darwin.json).
Language: Markdown
Created-at: 2026-03-06T14:51:00.000Z
Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
-->


# GitSense Chat CLI Templates Configuration

This directory contains the launch templates for editors and terminals used by the `gsc contract launch` command, as well as shell initialization scripts and help documentation.

## File Structure

The templates are organized by type and operating system to ensure compatibility and logical separation of concerns.



```
templates/
├── commands/       # Editor and terminal launch templates
│   ├── darwin.json
│   ├── linux.json
│   └── windows.json
├── help/           # Documentation text files
│   ├── welcome.txt
│   └── help.txt
└── shells/         # Shell initialization scripts
    └── ws/
        ├── darwin/
        │   └── init.sh
        ├── linux/
        │   └── init.sh
        └── windows/
            └── init.ps1
```

The CLI automatically loads the configuration file matching your current operating system from the `commands/` directory.

## How to Customize

1. Open the JSON file corresponding to your OS in the `commands/` subdirectory.
2. Edit the `editors` or `terminals` objects.
3. Save the file.
4. Restart your GitSense Chat session or CLI to apply changes.

## Template Format

Templates use a simple key-value pair where the key is the alias (used in `gsc contract create --editor <alias>`) and the value is the command string.

The `%s` placeholder in the command string will be replaced with the target file path or directory.

### Example: Adding a Custom Terminal

If you want to launch iTerm2 with Vim automatically on macOS, you can add a custom entry to `commands/darwin.json`:

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
