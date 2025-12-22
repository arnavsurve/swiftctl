package device

// Platform represents a target platform
type Platform string

const (
	PlatformIOS      Platform = "ios"
	PlatformMacOS    Platform = "macos"
	PlatformWatchOS  Platform = "watchos"
	PlatformTVOS     Platform = "tvos"
	PlatformVisionOS Platform = "visionos"
)

// DeviceType distinguishes simulators from physical devices
type DeviceType int

const (
	DeviceTypeSimulator DeviceType = iota
	DeviceTypePhysical
)

// DeviceState represents the current state of a device
type DeviceState string

const (
	StateShutdown  DeviceState = "Shutdown"
	StateBooted    DeviceState = "Booted"
	StateBooting   DeviceState = "Booting"
	StateShuttingDown DeviceState = "Shutting Down"
)

// Device represents a simulator or physical device
type Device struct {
	UDID        string      `json:"udid"`
	Name        string      `json:"name"`
	Type        DeviceType  `json:"type"`
	Platform    Platform    `json:"platform"`
	OSVersion   string      `json:"os_version"`
	State       DeviceState `json:"state"`
	IsAvailable bool        `json:"is_available"`
}

// Implement DisplayDevice interface for UI rendering
func (d *Device) GetName() string      { return d.Name }
func (d *Device) GetUDID() string      { return d.UDID }
func (d *Device) GetState() string     { return string(d.State) }
func (d *Device) GetOSVersion() string { return d.OSVersion }
func (d *Device) GetPlatform() string  { return string(d.Platform) }
