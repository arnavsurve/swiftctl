package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arnavsurve/swiftctl/internal/device"
	"github.com/arnavsurve/swiftctl/internal/process"
)

type Detector struct {
	runner *process.Runner
}

func NewDetector() *Detector {
	return &Detector{
		runner: process.NewRunner(),
	}
}

// Detect finds a Swift project in dir. Priority: workspace > xcodeproj > SPM.
func (d *Detector) Detect(dir string) (*ProjectInfo, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	if info, err := d.detectWorkspace(absDir); err == nil {
		return info, nil
	}

	if info, err := d.detectXcodeProj(absDir); err == nil {
		return info, nil
	}

	if info, err := d.detectSPM(absDir); err == nil {
		return info, nil
	}

	return nil, fmt.Errorf("no Swift project found in %s", dir)
}

func (d *Detector) detectWorkspace(dir string) (*ProjectInfo, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.xcworkspace"))
	if err != nil || len(matches) == 0 {
		return nil, fmt.Errorf("no workspace found")
	}

	workspacePath := matches[0]
	name := strings.TrimSuffix(filepath.Base(workspacePath), ".xcworkspace")

	info := &ProjectInfo{
		Type: ProjectTypeWorkspace,
		Path: workspacePath,
		Name: name,
	}

	d.populateSchemes(info)
	return info, nil
}

func (d *Detector) detectXcodeProj(dir string) (*ProjectInfo, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.xcodeproj"))
	if err != nil || len(matches) == 0 {
		return nil, fmt.Errorf("no xcodeproj found")
	}

	projPath := matches[0]
	name := strings.TrimSuffix(filepath.Base(projPath), ".xcodeproj")

	info := &ProjectInfo{
		Type: ProjectTypeXcodeProj,
		Path: projPath,
		Name: name,
	}

	d.populateSchemes(info)
	return info, nil
}

func (d *Detector) detectSPM(dir string) (*ProjectInfo, error) {
	packagePath := filepath.Join(dir, "Package.swift")
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no Package.swift found")
	}

	info := &ProjectInfo{
		Type:      ProjectTypeSPM,
		Path:      packagePath,
		Name:      filepath.Base(dir),
		Platforms: []device.Platform{device.PlatformMacOS},
	}

	d.populateSPMInfo(info, dir)
	return info, nil
}

func (d *Detector) populateSchemes(info *ProjectInfo) {
	ctx := context.Background()

	var args []string
	switch info.Type {
	case ProjectTypeWorkspace:
		args = []string{"-workspace", info.Path, "-list", "-json"}
	case ProjectTypeXcodeProj:
		args = []string{"-project", info.Path, "-list", "-json"}
	default:
		return
	}

	output, err := d.runner.RunSilent(ctx, "xcodebuild", args)
	if err != nil {
		return
	}

	var result struct {
		Project struct {
			Schemes []string `json:"schemes"`
			Targets []string `json:"targets"`
		} `json:"project"`
		Workspace struct {
			Schemes []string `json:"schemes"`
		} `json:"workspace"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return
	}

	if info.Type == ProjectTypeWorkspace {
		info.Schemes = result.Workspace.Schemes
	} else {
		info.Schemes = result.Project.Schemes
		for _, t := range result.Project.Targets {
			info.Targets = append(info.Targets, Target{Name: t})
		}
	}

	info.Platforms = d.inferPlatforms(info.Schemes)
}

func (d *Detector) populateSPMInfo(info *ProjectInfo, dir string) {
	ctx := context.Background()

	output, err := d.runner.RunSilent(ctx, "swift", []string{"package", "describe", "--type", "json"})
	if err != nil {
		return
	}

	var pkg struct {
		Name     string `json:"name"`
		Products []struct {
			Name string   `json:"name"`
			Type struct {
				Executable *struct{} `json:"executable"`
				Library    *struct{} `json:"library"`
			} `json:"type"`
		} `json:"products"`
		Targets []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"targets"`
	}

	if err := json.Unmarshal(output, &pkg); err != nil {
		return
	}

	info.Name = pkg.Name
	for _, p := range pkg.Products {
		info.Schemes = append(info.Schemes, p.Name)
	}
	for _, t := range pkg.Targets {
		info.Targets = append(info.Targets, Target{
			Name:        t.Name,
			ProductType: t.Type,
			Platform:    device.PlatformMacOS,
		})
	}
}

// inferPlatforms guesses platforms from scheme names. Defaults to iOS.
func (d *Detector) inferPlatforms(schemes []string) []device.Platform {
	platforms := make(map[device.Platform]bool)

	for _, s := range schemes {
		s = strings.ToLower(s)
		switch {
		case strings.Contains(s, "ios"):
			platforms[device.PlatformIOS] = true
		case strings.Contains(s, "macos") || strings.Contains(s, "mac"):
			platforms[device.PlatformMacOS] = true
		case strings.Contains(s, "watchos") || strings.Contains(s, "watch"):
			platforms[device.PlatformWatchOS] = true
		case strings.Contains(s, "tvos"):
			platforms[device.PlatformTVOS] = true
		case strings.Contains(s, "visionos") || strings.Contains(s, "vision"):
			platforms[device.PlatformVisionOS] = true
		}
	}

	if len(platforms) == 0 {
		platforms[device.PlatformIOS] = true
	}

	result := make([]device.Platform, 0, len(platforms))
	for p := range platforms {
		result = append(result, p)
	}
	return result
}
