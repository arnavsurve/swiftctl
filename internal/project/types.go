package project

import "github.com/arnavsurve/swiftctl/internal/device"

type ProjectType int

const (
	ProjectTypeUnknown   ProjectType = iota
	ProjectTypeXcodeProj
	ProjectTypeWorkspace
	ProjectTypeSPM
)

func (t ProjectType) String() string {
	switch t {
	case ProjectTypeXcodeProj:
		return "xcodeproj"
	case ProjectTypeWorkspace:
		return "workspace"
	case ProjectTypeSPM:
		return "spm"
	default:
		return "unknown"
	}
}

type ProjectInfo struct {
	Type      ProjectType       `json:"type"`
	Path      string            `json:"path"`
	Name      string            `json:"name"`
	Schemes   []string          `json:"schemes"`
	Targets   []Target          `json:"targets"`
	Platforms []device.Platform `json:"platforms"`
}

type Target struct {
	Name        string          `json:"name"`
	Platform    device.Platform `json:"platform"`
	BundleID    string          `json:"bundle_id"`
	ProductType string          `json:"product_type"`
}
