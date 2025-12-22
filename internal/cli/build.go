package cli

import (
	"fmt"
	"path/filepath"

	"github.com/arnavsurve/swiftctl/internal/build"
	"github.com/arnavsurve/swiftctl/internal/device"
	"github.com/arnavsurve/swiftctl/internal/project"
	"github.com/arnavsurve/swiftctl/internal/ui"
	"github.com/spf13/cobra"
)

func buildCmd() *cobra.Command {
	var (
		scheme      string
		config      string
		platform    string
		destination string
		clean       bool
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the project",
		Long:  `Build the Swift project using xcodebuild.`,
		Example: `  swiftctl build
  swiftctl build -s MyScheme
  swiftctl build -c release
  swiftctl build --platform ios
  swiftctl build --clean`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			renderer := ui.NewRenderer()

			detector := project.NewDetector()
			proj, err := detector.Detect(".")
			if err != nil {
				return fmt.Errorf("no project found: %w", err)
			}

			builder := build.NewBuilder(proj)

			cfg := build.Config{
				Scheme:      scheme,
				Destination: destination,
			}

			switch config {
			case "release", "Release":
				cfg.Configuration = build.ConfigRelease
			default:
				cfg.Configuration = build.ConfigDebug
			}

			if platform != "" {
				cfg.Platform = device.Platform(platform)
			} else if len(proj.Platforms) > 0 {
				cfg.Platform = proj.Platforms[0]
			}

			if clean {
				renderer.StartSpinner("Cleaning...")
				if err := builder.Clean(ctx, cfg); err != nil {
					renderer.StopSpinner(false)
					renderer.Warning("Clean failed: %v", err)
				} else {
					renderer.StopSpinner(true)
				}
			}

			schemeName := scheme
			if schemeName == "" && len(proj.Schemes) > 0 {
				schemeName = proj.Schemes[0]
			}
			if schemeName == "" {
				schemeName = proj.Name
			}

			renderer.StartSpinner("Building %s...", schemeName)

			events := make(chan build.Event, 100)
			done := make(chan struct{})

			var lastFile string
			var warningCount, errorCount int

			go func() {
				for ev := range events {
					switch ev.Type {
					case build.EventCompileFile:
						lastFile = filepath.Base(ev.File)
						renderer.StopSpinner(true)
						renderer.StartSpinner("Compiling %s...", lastFile)

					case build.EventWarning:
						warningCount++

					case build.EventError:
						errorCount++
						renderer.StopSpinner(false)
						renderer.Error("%s:%d: %s", filepath.Base(ev.File), ev.Line, ev.Message)
						renderer.StartSpinner("Building...")

					case build.EventLink:
						renderer.StopSpinner(true)
						renderer.StartSpinner("Linking %s...", ev.Message)

					case build.EventSign:
						renderer.StopSpinner(true)
						renderer.StartSpinner("Signing...")
					}
				}
				close(done)
			}()

			result, err := builder.Build(ctx, cfg, events)
			close(events)
			<-done

			renderer.StopSpinner(result != nil && result.Success)

			if err != nil {
				return fmt.Errorf("build failed: %w", err)
			}

			if result.Success {
				renderer.Success("Build succeeded in %.1fs", result.Duration.Seconds())
				if warningCount > 0 {
					renderer.Warning("%d warning(s)", warningCount)
				}
			} else {
				renderer.Error("Build failed with %d error(s)", errorCount)
				for i, e := range result.Errors {
					if i >= 5 {
						renderer.Info("... and %d more errors", len(result.Errors)-5)
						break
					}
					renderer.Info("  %s:%d: %s", filepath.Base(e.File), e.Line, e.Message)
				}
				return fmt.Errorf("build failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&scheme, "scheme", "s", "", "Scheme to build")
	cmd.Flags().StringVarP(&config, "configuration", "c", "debug", "Build configuration (debug/release)")
	cmd.Flags().StringVarP(&platform, "platform", "p", "", "Target platform (ios, macos, etc.)")
	cmd.Flags().StringVar(&destination, "destination", "", "Build destination (xcodebuild format)")
	cmd.Flags().BoolVar(&clean, "clean", false, "Clean before building")

	return cmd
}
