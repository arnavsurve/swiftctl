package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arnavsurve/swiftctl/internal/device"
)

// FindApp locates the .app bundle in DerivedData after a build.
func FindApp(projectName, scheme, configuration string, platform device.Platform) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	derivedData := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")

	// Find project folder (has hash suffix)
	pattern := filepath.Join(derivedData, projectName+"-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no DerivedData found for %s", projectName)
	}

	// Use most recently modified project folder
	var bestMatch string
	var bestTime int64
	for _, m := range matches {
		info, err := os.Stat(m)
		if err == nil && info.ModTime().Unix() > bestTime {
			bestTime = info.ModTime().Unix()
			bestMatch = m
		}
	}

	// Build products path
	sdk := platformToSDK(platform)
	if configuration == "" {
		configuration = "Debug"
	}
	productsDir := filepath.Join(bestMatch, "Build", "Products", configuration+"-"+sdk)

	apps, err := filepath.Glob(filepath.Join(productsDir, "*.app"))
	if err != nil {
		return "", err
	}

	if len(apps) == 0 {
		return "", fmt.Errorf("no .app found in %s", productsDir)
	}

	// Prefer app matching scheme name
	for _, app := range apps {
		name := strings.TrimSuffix(filepath.Base(app), ".app")
		if name == scheme {
			return app, nil
		}
	}

	return apps[0], nil
}

func platformToSDK(p device.Platform) string {
	switch p {
	case device.PlatformIOS:
		return "iphonesimulator"
	case device.PlatformTVOS:
		return "appletvsimulator"
	case device.PlatformWatchOS:
		return "watchsimulator"
	case device.PlatformVisionOS:
		return "xrsimulator"
	default:
		return "iphonesimulator"
	}
}
