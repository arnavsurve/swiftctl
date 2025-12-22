package device

type Platform string

const (
	PlatformIOS      Platform = "ios"
	PlatformMacOS    Platform = "macos"
	PlatformWatchOS  Platform = "watchos"
	PlatformTVOS     Platform = "tvos"
	PlatformVisionOS Platform = "visionos"
)

type DeviceType int

const (
	DeviceTypeSimulator DeviceType = iota
	DeviceTypePhysical
)

type DeviceState string

const (
	StateShutdown     DeviceState = "Shutdown"
	StateBooted       DeviceState = "Booted"
	StateBooting      DeviceState = "Booting"
	StateShuttingDown DeviceState = "Shutting Down"
)

type Device struct {
	UDID        string      `json:"udid"`
	Name        string      `json:"name"`
	Type        DeviceType  `json:"type"`
	Platform    Platform    `json:"platform"`
	OSVersion   string      `json:"os_version"`
	State       DeviceState `json:"state"`
	IsAvailable bool        `json:"is_available"`
}

