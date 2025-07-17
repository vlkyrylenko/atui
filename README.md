# 🔍 AWS IAM Role Explorer TUI

A terminal-based user interface (TUI) application for exploring AWS IAM roles and policies using Go and the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework.

## ✨ Features

- 📋 List IAM roles associated with your current AWS profile
- 🔗 View policies attached to each role with clear visual indicators
- 👀 Navigate through policy lists with improved visibility
- 📄 View policy JSON documents with syntax highlighting
- 🏷️ Visual distinction between AWS managed and Customer managed policies
- ⌨️ Navigate using keyboard shortcuts
- 🎨 Beautiful terminal UI with styling
- 🔄 Switch between AWS profiles seamlessly

## 📋 Prerequisites

- Go 1.21 or later
- AWS credentials configured (via `~/.aws/credentials` or environment variables)
- AWS permissions to read IAM roles and policies

## 📦 Installation

### Option 1: Install using `go install` (Recommended)

Install the latest version directly from GitHub:

```bash
go install github.com/vlkyrylenko/atui@latest
```

This will install the `atui` binary to your `$GOPATH/bin` directory. Make sure this directory is in your `$PATH`.

### Option 2: Download Pre-built Binaries

Visit the [Releases page](https://github.com/vlkyrylenko/atui/releases) and download the appropriate binary for your operating system:

- **Linux (x64)**: `atui-linux-amd64`
- **macOS (Intel)**: `atui-darwin-amd64` 
- **macOS (Apple Silicon)**: `atui-darwin-arm64`
- **Windows (x64)**: `atui-windows-amd64.exe`

After downloading, make the binary executable (Linux/macOS):
```bash
chmod +x atui-*
sudo mv atui-* /usr/local/bin/atui
```

### Option 3: Build from Source

Clone this repository and build the application:

```bash
git clone https://github.com/vlkyrylenko/atui.git
cd atui
go build -o atui
mv atui /usr/local/bin/atui
```

### Option 4: Cross-Platform Build with Make 🔧

For building across multiple platforms, use the provided Makefile:

```bash
# Build for current platform
make build

# Build for all platforms (Linux, macOS, Windows)
make build-all

# Build for specific platforms
make build-linux    # Linux amd64
make build-darwin   # macOS amd64 and arm64
make build-windows  # Windows amd64
go install github.com/vlkyrylenko/atui.git
# Create distribution packages
make dist

# Install to $GOPATH/bin
make install

# See all available commands
make help
```

The cross-platform builds will be created in the `dist/` directory with the following naming convention:
- `atui-linux-amd64`
- `atui-darwin-amd64` (Intel Mac)
- `atui-darwin-arm64` (Apple Silicon Mac)
- `atui-windows-amd64.exe`

## 🚀 Usage

Run the application:

```bash
atui
```

The application automatically loads your default AWS profile. To use a different profile, set the `AWS_PROFILE` environment variable:

```bash
AWS_PROFILE=dev atui
```

### ⌨️ Keyboard Controls

- **↑/k**: Move up
- **↓/j**: Move down
- **Enter**: Select/view item
- **Esc**: Go back to previous screen
- **p**: Switch AWS profiles
- **q/Ctrl+C**: Quit application

## 🖥️ Screenshots

### Main Role List
Navigate through your AWS IAM roles with a clean, organized interface.

### Policy Details
View detailed policy information with syntax-highlighted JSON documents.

## 🔧 Configuration

The application supports configuration through a config file. Place your configuration in:
- Linux: `~/.config/atui/config.yaml`
- macOS: `~/Library/Application Support/atui/config.yaml`
- Windows: `%APPDATA%\atui\config.yaml`

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### 🛠️ Development Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/vlkyrylenko/atui.git
   cd atui
   ```

2. **Set up the development environment**:
   ```bash
   make dev-setup  # Installs dependencies and development tools
   ```

3. **Run the application locally**:
   ```bash
   make run
   ```

### 📝 Development Guidelines

- **Code formatting**: Run `make fmt` before committing
- **Linting**: Run `make lint` to check code quality
- **Testing**: Run `make test` to execute all tests
- **Cross-platform testing**: Use `make build-all` to ensure builds work on all platforms

### 🐛 Bug Reports & Feature Requests

- Please use GitHub Issues to report bugs or request features
- Include your OS, Go version, and AWS configuration details when reporting bugs
- Provide steps to reproduce the issue

### 💡 Development Notes

- The application uses [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI framework
- Configuration is handled through the `config/` package
- AWS API interactions are in the main application file
- Color themes and styling can be customized via the config system

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) - A powerful little TUI framework
- Uses [Lipgloss](https://github.com/charmbracelet/lipgloss) for styling
- AWS SDK for Go v2
