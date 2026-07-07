# Dotty 📦✨

Dotty is an elegant, highly visual TUI (Terminal User Interface) application for managing your dotfiles and system configurations. Written in Go with Bubble Tea, Dotty makes it painless to install, update, and remove your favorite configurations without wrestling with symlinks manually.

## Features 🚀

- **Beautiful TrueColor UI**: A gorgeous terminal interface built with Lipgloss. Smooth gradients, responsive sizing, and modern paddings.
- **Package Management for Dotfiles**: Install your Neovim, Hyprland, Waybar, or Zsh configs with a single click.
- **Alternatives Support**: Multiple configurations for a single application? No problem. Dotty allows you to switch between them seamlessly.
- **Sync & Repair**: Automatically detects broken symlinks or missing targets and repairs them.
- **Git Native**: Dotty pulls configs directly from Git repositories.

## Installation 💻

You can build and install Dotty via Go:

```bash
git clone https://github.com/ionnixf/Dotty.git
cd Dotty
go build -o dotty ./cmd/dotty
sudo mv dotty /usr/local/bin/
```

## Usage 🛠

Run the interactive TUI simply by typing:

```bash
dotty
```

### Screens
- **Install**: Browse the package catalog and select configurations to install.
- **Update**: Update your installed configurations by pulling the latest changes from Git.
- **Remove**: Uninstall a configuration and safely remove its symlinks.
- **Installed**: View the status of currently installed packages and whether their symlinks are healthy.
- **Sync**: Check for broken symlinks and repair them across your system.
- **Import Existing**: Scan your `~/.config` directory and let Dotty manage pre-existing configurations.
- **Settings**: Change the UI theme (Light/Dark) or adjust update preferences.

## Configuration ⚙️

Dotty manages an internal package catalog (`packages.json`). You can customize the default target paths, repository URLs, and descriptions inside `pkg/configs/packages.json`.

## License 📄

This project is licensed under the MIT License.
