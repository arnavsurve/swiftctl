package cli

import (
	"fmt"

	"github.com/arnavsurve/swiftctl/internal/build"
	"github.com/arnavsurve/swiftctl/internal/device"
	"github.com/arnavsurve/swiftctl/internal/project"
	"github.com/arnavsurve/swiftctl/internal/run"
	"github.com/arnavsurve/swiftctl/internal/ui"
	"github.com/spf13/cobra"
)

func runCmd() *cobra.Command {
	var (
		scheme        string
		configuration string
		deviceName    string
		watch         bool
		launchArgs    []string
	)

	cmd := &cobra.Command{
		Use:   "run <platform>",
		Short: "Build, deploy, and run on simulator",
		Long: `Build the project, boot a simulator, install the app, launch it, and stream logs.

Use -w/--watch to automatically rebuild and relaunch when source files change.`,
		Example: `  swiftctl run ios
  swiftctl run ios -w
  swiftctl run ios -s MyScheme -d "iPhone 15 Pro"
  swiftctl run ios -c release
  swiftctl run ios --args="-verbose,-debug"`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"ios", "watchos", "tvos", "visionos"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			renderer := ui.NewRenderer()

			platform := device.Platform(args[0])

			switch platform {
			case device.PlatformIOS, device.PlatformWatchOS, device.PlatformTVOS, device.PlatformVisionOS:
				// Valid
			case device.PlatformMacOS:
				return fmt.Errorf("use 'swiftctl build' for macOS, then run the binary directly")
			default:
				return fmt.Errorf("unknown platform: %s (valid: ios, watchos, tvos, visionos)", args[0])
			}

			detector := project.NewDetector()
			proj, err := detector.Detect(".")
			if err != nil {
				return fmt.Errorf("no project found: %w", err)
			}

			renderer.Info("Project: %s (%s)", proj.Name, proj.Type)

			cfg := run.Config{
				Scheme:     scheme,
				Platform:   platform,
				DeviceName: deviceName,
				Watch:      watch,
				LaunchArgs: launchArgs,
			}

			switch configuration {
			case "release", "Release":
				cfg.Configuration = build.ConfigRelease
			default:
				cfg.Configuration = build.ConfigDebug
			}

			runner := run.NewRunner(proj)
			return runner.Run(ctx, cfg)
		},
	}

	cmd.Flags().StringVarP(&scheme, "scheme", "s", "", "Scheme to build (default: first available)")
	cmd.Flags().StringVarP(&configuration, "configuration", "c", "debug", "Build configuration (debug/release)")
	cmd.Flags().StringVarP(&deviceName, "device", "d", "", "Target device name or UDID")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for file changes and rebuild")
	cmd.Flags().StringSliceVar(&launchArgs, "args", nil, "Arguments to pass to the launched app")

	return cmd
}
