# DailyLog

I was having difficulty remembering things I had completed from day to day, so I created this TUI app for logging daily completed tasks. The app creates timestamped entries in markdown files, organized by date.

## Features

-   Log complete tasks with a timestamp
-   Vim-motion navigation (h/j/k/l)
-   Daily logs are stored as markdown files
-   Allows for snapshots to be created as zips of the logs directory
-   Uses Bubble Tea for the UI

## Installation

### Prerequisites

1. Install Go (1.21 or later):
    - **Windows**: Download and install from [Go's official website](https://golang.org/dl/)
    - **Mac**: Using Homebrew: `brew install go`

### Installing DailyLog

1. Clone the repository:

    ```bash
    # Using HTTPS:
    git clone https://github.com/seanblankenship/dailylog.git
    # Or using SSH if you have it configured:
    git clone git@github.com:seanblankenship/dailylog.git
    ```

2. Navigate to the project directory:

    ```bash
    cd dailylog
    ```

3. Build and install the application:
    ```bash
    go install
    ```

### Post-Installation Setup

#### Windows

1. Ensure `%GOPATH%\bin` is in your PATH environment variable
2. Open Command Prompt or PowerShell and run `dailylog`

#### Mac/Linux

1. Ensure `$GOPATH/bin` is in your PATH. Add this to your `~/.bashrc` or `~/.zshrc`:
    ```bash
    export PATH=$PATH:$(go env GOPATH)/bin
    ```
2. Open terminal and run `dailylog`

## Usage

### Key Bindings

-   `a` - Add a new note
-   `enter` - View selected note
-   `j/k` or `up/down` - Navigate through notes
-   `B` - Create a backup of all notes
-   `esc` - Return to previous screen
-   `q` - Quit application

### File Locations

-   logs are stored in `~/.dailylog/logs/` as markdown files
-   backups are stored in `~/.dailylog/backups/` as ZIP files

## Changelog

### 2/27/25

-   reversed the order of entries when starting the app so that most recent shows at the top
