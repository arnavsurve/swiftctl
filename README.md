# swiftctl

A command-line tool for Swift development on macOS. Written in Go.

## What it does

- Detects Xcode projects, workspaces, and Swift packages
- Lists, boots, and shuts down iOS/macOS/watchOS/tvOS/visionOS simulators
- Builds projects via xcodebuild

## Requirements

- macOS
- Xcode with Command Line Tools
- Go 1.21+ (to build from source)

## Installation

```bash
./install.sh
```

This builds the binary and copies it to `/usr/local/bin/swiftctl`.

To install elsewhere:

```bash
INSTALL_DIR=~/bin ./install.sh
```

## Usage

### List simulators

```bash
swiftctl devices list
swiftctl devices list --platform ios
swiftctl devices list --booted
swiftctl devices list --json
```

### Boot a simulator

```bash
swiftctl devices boot "iPhone 15 Pro"
```

### Shut down simulators

```bash
swiftctl devices shutdown "iPhone 15 Pro"
swiftctl devices shutdown all
```

### Build a project

```bash
swiftctl build
swiftctl build --scheme MyApp
swiftctl build --scheme MyApp --configuration release
swiftctl build --clean
```

### View project info

```bash
swiftctl project info
swiftctl project info --json
```

### Global flags

```bash
swiftctl --verbose <command>  # Show underlying commands
swiftctl --help               # Show help
swiftctl --version            # Show version
```

## Project detection

swiftctl looks for projects in this order:

1. `*.xcworkspace`
2. `*.xcodeproj`
3. `Package.swift`

## License

MIT
