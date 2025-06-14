# AWS IAM Role Explorer TUI

A terminal-based user interface (TUI) application for exploring AWS IAM roles and policies using Go and the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework.

## Features

- List IAM roles associated with your current AWS profile
- View policies attached to each role
- View and open policy JSON documents in your default text editor
- Navigate using keyboard shortcuts
- Beautiful terminal UI with styling

## Prerequisites

- Go 1.18 or later
- AWS credentials configured (via `~/.aws/credentials` or environment variables)
- AWS permissions to read IAM roles and policies

## Installation

Clone this repository and build the application:

```bash
git clone <repository-url>
cd atui
go build
```

## Usage

Run the application:

```bash
./atui
```

The application automatically loads your default AWS profile. To use a different profile, set the `AWS_PROFILE` environment variable:

```bash
AWS_PROFILE=dev ./atui
```

### Keyboard Controls

- **↑/k**: Move up
- **↓/j**: Move down
- **Enter**: Select/view item
- **Esc/Backspace**: Go back
- **o**: Open policy JSON in default editor (when viewing a policy)
- **q/Ctrl+C**: Quit application

## Workflow

1. The app shows a list of IAM roles you have access to
2. Select a role to see its attached policies
3. Select a policy to view its JSON document
4. Press 'o' to open the JSON in your default editor

## Development

To modify or extend this application:

```bash
# Install dependencies
go get github.com/charmbracelet/bubbletea github.com/charmbracelet/bubbles github.com/charmbracelet/lipgloss
go get github.com/aws/aws-sdk-go-v2 github.com/aws/aws-sdk-go-v2/config github.com/aws/aws-sdk-go-v2/service/iam github.com/aws/aws-sdk-go-v2/service/sts
```
