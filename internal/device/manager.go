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
	output, err := m.runner.RunSilent(ctx, "xcrun", []string{"simctl", "list", "devices", "-j"})
	if err != nil {
		return nil, fmt.Errorf("simctl list: %w", err)
	}

	var devices []*Device

	gjson.ParseBytes(output).Get("devices").ForEach(func(runtime, devicesArray gjson.Result) bool {
		plat, version := parseRuntime(runtime.String())
		if platform != "" && plat != platform {
			return true
		}

		devicesArray.ForEach(func(_, dev gjson.Result) bool {
			if !dev.Get("isAvailable").Bool() {
				return true
			}

			state := DeviceState(dev.Get("state").String())
			if onlyBooted && state != StateBooted {
				return true
			}

			devices = append(devices, &Device{
				UDID:        dev.Get("udid").String(),
				Name:        dev.Get("name").String(),
				Type:        DeviceTypeSimulator,
				Platform:    plat,
				OSVersion:   version,
				State:       state,
				IsAvailable: true,
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

	for _, d := range devices {
		if d.UDID == nameOrUDID {
			return d, nil
		}
	}

	nameOrUDID = strings.ToLower(nameOrUDID)
	for _, d := range devices {
		if strings.ToLower(d.Name) == nameOrUDID {
			return d, nil
		}
	}

	for _, d := range devices {
		if strings.Contains(strings.ToLower(d.Name), nameOrUDID) {
			return d, nil
		}
	}

	return nil, fmt.Errorf("device not found: %s", nameOrUDID)
}

func (m *Manager) Boot(ctx context.Context, device *Device) error {
	if device.State == StateBooted {
		return nil
	}

	_, err := m.runner.RunSilent(ctx, "xcrun", []string{"simctl", "boot", device.UDID})
	if err != nil {
		return fmt.Errorf("boot %s: %w", device.Name, err)
	}

	_, _ = m.runner.RunSilent(ctx, "open", []string{"-a", "Simulator"})

	return nil
}

func (m *Manager) Shutdown(ctx context.Context, device *Device) error {
	if device.State == StateShutdown {
		return nil
	}

	_, err := m.runner.RunSilent(ctx, "xcrun", []string{"simctl", "shutdown", device.UDID})
	if err != nil {
		return fmt.Errorf("shutdown %s: %w", device.Name, err)
	}

	return nil
}

func (m *Manager) ShutdownAll(ctx context.Context) error {
	_, err := m.runner.RunSilent(ctx, "xcrun", []string{"simctl", "shutdown", "all"})
	return err
}

func (m *Manager) Install(ctx context.Context, device *Device, appPath string) error {
	_, err := m.runner.RunSilent(ctx, "xcrun", []string{"simctl", "install", device.UDID, appPath})
	if err != nil {
		return fmt.Errorf("install on %s: %w", device.Name, err)
	}
	return nil
}

func (m *Manager) Launch(ctx context.Context, device *Device, bundleID string, args []string) (int, error) {
	cmdArgs := []string{"simctl", "launch", device.UDID, bundleID}
	cmdArgs = append(cmdArgs, args...)

	output, err := m.runner.RunSilent(ctx, "xcrun", cmdArgs)
	if err != nil {
		return 0, fmt.Errorf("launch %s: %w", bundleID, err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ": ")
	if len(parts) == 2 {
		var pid int
		fmt.Sscanf(parts[1], "%d", &pid)
		return pid, nil
	}

	return 0, nil
}

func (m *Manager) Terminate(ctx context.Context, device *Device, bundleID string) error {
	m.runner.RunSilent(ctx, "xcrun", []string{"simctl", "terminate", device.UDID, bundleID})
	return nil
}

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

	version := ""
	parts := strings.Split(runtime, "-")
	if len(parts) >= 2 {
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
