# swiftctl

A CLI toolkit for Swift development on macOS.

## What it does

- Detects Xcode projects, workspaces, and Swift packages
- Builds, deploys, launches, and streams logs from apps
- Manages iOS/macOS/watchOS/tvOS/visionOS simulators
- Watches for file changes and automatically rebuilds

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

### Run an app

Build, deploy to simulator, launch, and stream logs:

```bash
swiftctl run ios
swiftctl run ios -w                        # Watch mode: rebuild on file changes
swiftctl run ios -s MyScheme               # Specify scheme
swiftctl run ios -d "iPhone 15 Pro"        # Specify device
swiftctl run ios -c release                # Release configuration
swiftctl run ios --args="-debug,-verbose"  # Pass args to app
```

### Build a project

```bash
swiftctl build
swiftctl build --scheme MyApp
swiftctl build --scheme MyApp --configuration release
swiftctl build --clean
```

### List simulators

```bash
swiftctl devices list
swiftctl devices list --platform ios
swiftctl devices list --booted
swiftctl devices list --json
```

### Boot and shutdown simulators

```bash
swiftctl devices boot "iPhone 15 Pro"
swiftctl devices shutdown "iPhone 15 Pro"
swiftctl devices shutdown all
```

### Create and delete simulators

```bash
swiftctl devices create "My iPhone" "iPhone 15 Pro" "iOS 17.0"
swiftctl devices delete "My iPhone"
```

### List available device types and runtimes

```bash
swiftctl devices types
swiftctl devices types --platform ios
swiftctl devices runtimes
swiftctl devices runtimes --platform ios
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
