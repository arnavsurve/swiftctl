package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/asurve/swiftctl/internal/device"
	"github.com/asurve/swiftctl/internal/ui"
	"github.com/spf13/cobra"
)

func devicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Manage simulators and devices",
		Long:  `List, boot, and shutdown iOS/macOS simulators.`,
	}

	cmd.AddCommand(devicesListCmd())
	cmd.AddCommand(devicesBootCmd())
	cmd.AddCommand(devicesShutdownCmd())

	return cmd
}

func devicesListCmd() *cobra.Command {
	var (
		platform  string
		booted    bool
		jsonOut   bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available simulators",
		Example: `  swiftctl devices list
  swiftctl devices list --platform ios
  swiftctl devices list --booted
  swiftctl devices list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			mgr := device.NewManager()

			devices, err := mgr.List(ctx, device.Platform(platform), booted)
			if err != nil {
				return fmt.Errorf("failed to list devices: %w", err)
			}

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(devices)
			}

			// Convert to UI types
			displayDevices := make([]ui.DeviceInfo, len(devices))
			for i, d := range devices {
				displayDevices[i] = ui.DeviceInfo{
					Name:      d.Name,
					UDID:      d.UDID,
					State:     string(d.State),
					OSVersion: d.OSVersion,
					Platform:  string(d.Platform),
				}
			}

			renderer := ui.NewRenderer()
			renderer.RenderDeviceList(displayDevices)
			return nil
		},
	}

	cmd.Flags().StringVarP(&platform, "platform", "p", "", "Filter by platform (ios, macos, watchos, tvos, visionos)")
	cmd.Flags().BoolVar(&booted, "booted", false, "Show only booted devices")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

func devicesBootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "boot <device>",
		Short: "Boot a simulator",
		Long:  `Boot a simulator by name or UDID.`,
		Example: `  swiftctl devices boot "iPhone 15 Pro"
  swiftctl devices boot 12345678-1234-1234-1234-123456789ABC`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			mgr := device.NewManager()
			renderer := ui.NewRenderer()

			dev, err := mgr.Get(ctx, args[0])
			if err != nil {
				return fmt.Errorf("device not found: %w", err)
			}

			renderer.StartSpinner(fmt.Sprintf("Booting %s...", dev.Name))

			if err := mgr.Boot(ctx, dev); err != nil {
				renderer.StopSpinner(false)
				return fmt.Errorf("failed to boot: %w", err)
			}

			renderer.StopSpinner(true)
			renderer.Success("Booted %s", dev.Name)
			return nil
		},
	}
}

func devicesShutdownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shutdown [device|all]",
		Short: "Shutdown simulator(s)",
		Long:  `Shutdown a specific simulator or all running simulators.`,
		Example: `  swiftctl devices shutdown all
  swiftctl devices shutdown "iPhone 15 Pro"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			mgr := device.NewManager()
			renderer := ui.NewRenderer()

			// Shutdown all if no arg or "all"
			if len(args) == 0 || args[0] == "all" {
				renderer.StartSpinner("Shutting down all simulators...")
				if err := mgr.ShutdownAll(ctx); err != nil {
					renderer.StopSpinner(false)
					return fmt.Errorf("failed to shutdown: %w", err)
				}
				renderer.StopSpinner(true)
				renderer.Success("All simulators shut down")
				return nil
			}

			// Shutdown specific device
			dev, err := mgr.Get(ctx, args[0])
			if err != nil {
				return fmt.Errorf("device not found: %w", err)
			}

			renderer.StartSpinner(fmt.Sprintf("Shutting down %s...", dev.Name))
			if err := mgr.Shutdown(ctx, dev); err != nil {
				renderer.StopSpinner(false)
				return fmt.Errorf("failed to shutdown: %w", err)
			}

			renderer.StopSpinner(true)
			renderer.Success("Shut down %s", dev.Name)
			return nil
		},
	}
}
