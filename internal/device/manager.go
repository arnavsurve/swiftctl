package device

import (
	"context"
	"fmt"
	"strings"

	"github.com/asurve/swiftctl/internal/process"
	"github.com/tidwall/gjson"
)

// Manager handles simulator and device operations
type Manager struct {
	runner *process.Runner
}

// NewManager creates a new device Manager
func NewManager() *Manager {
	return &Manager{
		runner: process.NewRunner(),
	}
}

// List returns available devices, optionally filtered by platform and state
func (m *Manager) List(ctx context.Context, platform Platform, onlyBooted bool) ([]*Device, error) {
	// Get simulator list as JSON
	output, err := m.runner.RunSilent(ctx, "xcrun", []string{"simctl", "list", "devices", "-j"})
	if err != nil {
		return nil, fmt.Errorf("simctl list: %w", err)
	}

	var devices []*Device

	// Parse JSON using gjson for flexibility
	result := gjson.ParseBytes(output)

	// Iterate over device runtimes
	result.Get("devices").ForEach(func(runtime, devicesArray gjson.Result) bool {
		// Extract platform and version from runtime string
		// e.g., "com.apple.CoreSimulator.SimRuntime.iOS-17-0" -> "ios", "17.0"
		plat, version := parseRuntime(runtime.String())

		// Filter by platform if specified
		if platform != "" && plat != platform {
			return true // continue
		}

		// Iterate over devices in this runtime
		devicesArray.ForEach(func(_, dev gjson.Result) bool {
			isAvailable := dev.Get("isAvailable").Bool()
			if !isAvailable {
				return true // skip unavailable
			}

			state := DeviceState(dev.Get("state").String())
			if onlyBooted && state != StateBooted {
				return true // skip non-booted
			}

			devices = append(devices, &Device{
				UDID:        dev.Get("udid").String(),
				Name:        dev.Get("name").String(),
				Type:        DeviceTypeSimulator,
				Platform:    plat,
				OSVersion:   version,
				State:       state,
				IsAvailable: isAvailable,
			})
			return true
		})
		return true
	})

	return devices, nil
}

// Get finds a device by name or UDID
func (m *Manager) Get(ctx context.Context, nameOrUDID string) (*Device, error) {
	devices, err := m.List(ctx, "", false)
	if err != nil {
		return nil, err
	}

	// Try exact UDID match first
	for _, d := range devices {
		if d.UDID == nameOrUDID {
			return d, nil
		}
	}

	// Try name match (case-insensitive)
	nameOrUDID = strings.ToLower(nameOrUDID)
	for _, d := range devices {
		if strings.ToLower(d.Name) == nameOrUDID {
			return d, nil
		}
	}

	// Try partial name match
	for _, d := range devices {
		if strings.Contains(strings.ToLower(d.Name), nameOrUDID) {
			return d, nil
		}
	}

	return nil, fmt.Errorf("device not found: %s", nameOrUDID)
}

// Boot starts a simulator
func (m *Manager) Boot(ctx context.Context, device *Device) error {
	if device.State == StateBooted {
		return nil // Already booted
	}

	_, err := m.runner.RunSilent(ctx, "xcrun", []string{"simctl", "boot", device.UDID})
	if err != nil {
		return fmt.Errorf("boot %s: %w", device.Name, err)
	}

	// Open Simulator.app to show the device
	_, _ = m.runner.RunSilent(ctx, "open", []string{"-a", "Simulator"})

	return nil
}

// Shutdown stops a simulator
func (m *Manager) Shutdown(ctx context.Context, device *Device) error {
	if device.State == StateShutdown {
		return nil // Already shutdown
	}

	_, err := m.runner.RunSilent(ctx, "xcrun", []string{"simctl", "shutdown", device.UDID})
	if err != nil {
		return fmt.Errorf("shutdown %s: %w", device.Name, err)
	}

	return nil
}

// ShutdownAll stops all running simulators
func (m *Manager) ShutdownAll(ctx context.Context) error {
	_, err := m.runner.RunSilent(ctx, "xcrun", []string{"simctl", "shutdown", "all"})
	return err
}

// Install installs an app on a device
func (m *Manager) Install(ctx context.Context, device *Device, appPath string) error {
	_, err := m.runner.RunSilent(ctx, "xcrun", []string{"simctl", "install", device.UDID, appPath})
	if err != nil {
		return fmt.Errorf("install on %s: %w", device.Name, err)
	}
	return nil
}

// Launch starts an app and returns its PID
func (m *Manager) Launch(ctx context.Context, device *Device, bundleID string, args []string) (int, error) {
	cmdArgs := []string{"simctl", "launch", device.UDID, bundleID}
	cmdArgs = append(cmdArgs, args...)

	output, err := m.runner.RunSilent(ctx, "xcrun", cmdArgs)
	if err != nil {
		return 0, fmt.Errorf("launch %s: %w", bundleID, err)
	}

	// Parse PID from output: "com.app.MyApp: 12345"
	parts := strings.Split(strings.TrimSpace(string(output)), ": ")
	if len(parts) == 2 {
		var pid int
		fmt.Sscanf(parts[1], "%d", &pid)
		return pid, nil
	}

	return 0, nil
}

// Terminate kills a running app
func (m *Manager) Terminate(ctx context.Context, device *Device, bundleID string) error {
	_, err := m.runner.RunSilent(ctx, "xcrun", []string{"simctl", "terminate", device.UDID, bundleID})
	// Ignore error if app isn't running
	_ = err
	return nil
}

// parseRuntime extracts platform and version from a runtime identifier
// e.g., "com.apple.CoreSimulator.SimRuntime.iOS-17-0" -> PlatformIOS, "17.0"
func parseRuntime(runtime string) (Platform, string) {
	runtime = strings.ToLower(runtime)

	var platform Platform
	switch {
	case strings.Contains(runtime, "ios"):
		platform = PlatformIOS
	case strings.Contains(runtime, "macos"):
		platform = PlatformMacOS
	case strings.Contains(runtime, "watchos"):
		platform = PlatformWatchOS
	case strings.Contains(runtime, "tvos"):
		platform = PlatformTVOS
	case strings.Contains(runtime, "xros"), strings.Contains(runtime, "visionos"):
		platform = PlatformVisionOS
	default:
		platform = Platform("unknown")
	}

	// Extract version: "iOS-17-0" -> "17.0"
	version := ""
	parts := strings.Split(runtime, "-")
	if len(parts) >= 2 {
		// Find the version numbers at the end
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i][0] >= '0' && parts[i][0] <= '9' {
				if version == "" {
					version = parts[i]
				} else {
					version = parts[i] + "." + version
				}
			} else {
				break
			}
		}
	}

	return platform, version
}
