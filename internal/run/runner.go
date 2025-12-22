package run

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/arnavsurve/swiftctl/internal/build"
	"github.com/arnavsurve/swiftctl/internal/device"
	"github.com/arnavsurve/swiftctl/internal/process"
	"github.com/arnavsurve/swiftctl/internal/project"
	"github.com/arnavsurve/swiftctl/internal/ui"
	"github.com/arnavsurve/swiftctl/internal/watcher"
)

type Config struct {
	Scheme        string
	Configuration build.Configuration
	DeviceName    string
	Platform      device.Platform
	Watch         bool
	LaunchArgs    []string
}

type Runner struct {
	project       *project.ProjectInfo
	deviceManager *device.Manager
	builder       *build.Builder
	renderer      *ui.Renderer
	procRunner    *process.Runner
}

func NewRunner(proj *project.ProjectInfo) *Runner {
	return &Runner{
		project:       proj,
		deviceManager: device.NewManager(),
		builder:       build.NewBuilder(proj),
		renderer:      ui.NewRenderer(),
		procRunner:    process.NewRunner(),
	}
}

func (r *Runner) Run(ctx context.Context, cfg Config) error {
	// Resolve device
	dev, err := r.resolveDevice(ctx, cfg)
	if err != nil {
		return err
	}
	r.renderer.Info("Device: %s (%s)", dev.Name, dev.OSVersion)

	// Initial build cycle
	appPath, bundleID, err := r.buildCycle(ctx, cfg, dev)
	if err != nil {
		return err
	}

	if cfg.Watch {
		return r.runWithWatch(ctx, cfg, dev, appPath, bundleID)
	}

	return r.streamLogs(ctx, dev, bundleID)
}

func (r *Runner) resolveDevice(ctx context.Context, cfg Config) (*device.Device, error) {
	if cfg.DeviceName != "" {
		dev, err := r.deviceManager.Get(ctx, cfg.DeviceName)
		if err != nil {
			return nil, fmt.Errorf("device not found: %w", err)
		}
		return dev, nil
	}

	// Find suitable device for platform
	devices, err := r.deviceManager.List(ctx, cfg.Platform, false)
	if err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no %s simulators found (try: swiftctl devices list)", cfg.Platform)
	}

	// Prefer already booted
	for _, d := range devices {
		if d.State == device.StateBooted {
			return d, nil
		}
	}

	return devices[0], nil
}

// buildCycle performs build -> boot -> install -> launch
func (r *Runner) buildCycle(ctx context.Context, cfg Config, dev *device.Device) (appPath, bundleID string, err error) {
	scheme := cfg.Scheme
	if scheme == "" && len(r.project.Schemes) > 0 {
		scheme = r.project.Schemes[0]
	}

	// Build
	r.renderer.StartSpinner("Building %s...", scheme)

	buildCfg := build.Config{
		Scheme:        scheme,
		Configuration: cfg.Configuration,
		Platform:      cfg.Platform,
		Destination:   fmt.Sprintf("platform=iOS Simulator,id=%s", dev.UDID),
	}

	events := make(chan build.Event, 100)
	done := make(chan struct{})
	var lastFile string

	go func() {
		for ev := range events {
			switch ev.Type {
			case build.EventCompileFile:
				lastFile = filepath.Base(ev.File)
				r.renderer.StopSpinner(true)
				r.renderer.StartSpinner("Compiling %s...", lastFile)
			case build.EventError:
				r.renderer.StopSpinner(false)
				r.renderer.Error("%s:%d: %s", filepath.Base(ev.File), ev.Line, ev.Message)
				r.renderer.StartSpinner("Building...")
			}
		}
		close(done)
	}()

	result, buildErr := r.builder.Build(ctx, buildCfg, events)
	close(events)
	<-done

	if buildErr != nil {
		r.renderer.StopSpinner(false)
		return "", "", fmt.Errorf("build failed: %w", buildErr)
	}

	if result == nil || !result.Success {
		r.renderer.StopSpinner(false)
		errCount := 0
		if result != nil {
			errCount = len(result.Errors)
		}
		return "", "", fmt.Errorf("build failed with %d errors", errCount)
	}

	r.renderer.StopSpinner(true)
	r.renderer.Success("Built in %.1fs", result.Duration.Seconds())

	// Find .app
	config := string(cfg.Configuration)
	if config == "" {
		config = "Debug"
	}
	appPath, err = FindApp(r.project.Name, scheme, config, cfg.Platform)
	if err != nil {
		return "", "", fmt.Errorf("app not found: %w", err)
	}

	// Extract bundle ID
	bundleID, err = r.extractBundleID(appPath)
	if err != nil {
		return "", "", fmt.Errorf("bundle ID extraction failed: %w", err)
	}

	// Boot device
	if dev.State != device.StateBooted {
		r.renderer.StartSpinner("Booting %s...", dev.Name)
		if err := r.deviceManager.Boot(ctx, dev); err != nil {
			r.renderer.StopSpinner(false)
			return "", "", fmt.Errorf("boot failed: %w", err)
		}
		r.renderer.StopSpinner(true)
	}

	// Install
	r.renderer.StartSpinner("Installing...")
	if err := r.deviceManager.Install(ctx, dev, appPath); err != nil {
		r.renderer.StopSpinner(false)
		return "", "", fmt.Errorf("install failed: %w", err)
	}
	r.renderer.StopSpinner(true)

	// Terminate existing instance
	_ = r.deviceManager.Terminate(ctx, dev, bundleID)

	// Launch
	r.renderer.StartSpinner("Launching...")
	pid, err := r.deviceManager.Launch(ctx, dev, bundleID, cfg.LaunchArgs)
	if err != nil {
		r.renderer.StopSpinner(false)
		return "", "", fmt.Errorf("launch failed: %w", err)
	}
	r.renderer.StopSpinner(true)
	r.renderer.Success("Launched (PID %d)", pid)

	return appPath, bundleID, nil
}

func (r *Runner) streamLogs(ctx context.Context, dev *device.Device, bundleID string) error {
	r.renderer.Dim("Streaming logs (Ctrl+C to stop)...")

	streamer := NewLogStreamer(dev, bundleID)
	logs, errs := streamer.Stream(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case line, ok := <-logs:
			if !ok {
				return nil
			}
			fmt.Println(line)
		case err := <-errs:
			if err != nil {
				return err
			}
		}
	}
}

func (r *Runner) runWithWatch(ctx context.Context, cfg Config, dev *device.Device, appPath, bundleID string) error {
	w, err := watcher.New(750 * time.Millisecond)
	if err != nil {
		return fmt.Errorf("watcher failed: %w", err)
	}
	defer w.Close()

	if err := w.AddRecursive("."); err != nil {
		return fmt.Errorf("watch directory failed: %w", err)
	}

	changes := w.Watch(ctx)

	// Track current cancel function for cleanup
	var currentCancel context.CancelFunc

	cleanup := func() {
		if currentCancel != nil {
			currentCancel()
		}
	}
	defer cleanup()

	startLogs := func(bid string) {
		cleanup()
		var logCtx context.Context
		logCtx, currentCancel = context.WithCancel(ctx)
		streamer := NewLogStreamer(dev, bid)
		logs, _ := streamer.Stream(logCtx)

		go func() {
			for line := range logs {
				fmt.Println(line)
			}
		}()
	}

	startLogs(bundleID)
	r.renderer.Dim("Watching for changes (Ctrl+C to stop)...")

	for {
		select {
		case <-ctx.Done():
			return nil

		case change, ok := <-changes:
			if !ok {
				return nil
			}

			r.renderer.Info("Changed: %s", filepath.Base(change.Path))

			// Stop log streaming (app keeps running until build succeeds)
			cleanup()

			// Rebuild (buildCycle will terminate old app after successful build)
			newAppPath, newBundleID, err := r.buildCycle(ctx, cfg, dev)
			if err != nil {
				r.renderer.Error("Rebuild failed: %v", err)
				startLogs(bundleID)
				continue
			}

			appPath = newAppPath
			bundleID = newBundleID

			// Drain any queued events (from atomic saves generating multiple events)
			drainDone := time.After(100 * time.Millisecond)
		drain:
			for {
				select {
				case <-changes:
					// Discard queued events
				case <-drainDone:
					break drain
				}
			}

			// Restart log streaming
			startLogs(bundleID)
		}
	}
}

func (r *Runner) extractBundleID(appPath string) (string, error) {
	plistPath := filepath.Join(appPath, "Info.plist")
	output, err := r.procRunner.RunSilent(context.Background(), "/usr/libexec/PlistBuddy",
		[]string{"-c", "Print :CFBundleIdentifier", plistPath})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
