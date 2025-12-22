package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/arnavsurve/swiftctl/internal/device"
	"github.com/arnavsurve/swiftctl/internal/ui"
	"github.com/spf13/cobra"
)

func devicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Manage simulators and devices",
		Long:  `List, boot, shutdown, and create iOS/macOS simulators.`,
	}

	cmd.AddCommand(devicesListCmd())
	cmd.AddCommand(devicesBootCmd())
	cmd.AddCommand(devicesShutdownCmd())
	cmd.AddCommand(devicesCreateCmd())
	cmd.AddCommand(devicesDeleteCmd())
	cmd.AddCommand(devicesTypesCmd())
	cmd.AddCommand(devicesRuntimesCmd())

	return cmd
}

func devicesListCmd() *cobra.Command {
	var (
		platform string
		booted   bool
		jsonOut  bool
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

			renderer.StartSpinner("Booting %s...", dev.Name)

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

			dev, err := mgr.Get(ctx, args[0])
			if err != nil {
				return fmt.Errorf("device not found: %w", err)
			}

			renderer.StartSpinner("Shutting down %s...", dev.Name)
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

func devicesCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name> <device-type> <runtime>",
		Short: "Create a new simulator",
		Example: `  swiftctl devices create "My iPhone" "iPhone 15 Pro" "iOS 17.0"
  swiftctl devices create "Test Phone" com.apple.CoreSimulator.SimDeviceType.iPhone-15-Pro com.apple.CoreSimulator.SimRuntime.iOS-17-0`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			mgr := device.NewManager()
			renderer := ui.NewRenderer()

			name, deviceType, runtime := args[0], args[1], args[2]

			deviceTypeID, err := resolveDeviceType(ctx, mgr, deviceType)
			if err != nil {
				return err
			}

			runtimeID, err := resolveRuntime(ctx, mgr, runtime)
			if err != nil {
				return err
			}

			renderer.StartSpinner("Creating %s...", name)
			udid, err := mgr.Create(ctx, name, deviceTypeID, runtimeID)
			if err != nil {
				renderer.StopSpinner(false)
				return fmt.Errorf("failed to create: %w", err)
			}

			renderer.StopSpinner(true)
			renderer.Success("Created %s (%s)", name, udid)
			return nil
		},
	}
}

func devicesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <device>",
		Short: "Delete a simulator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			mgr := device.NewManager()
			renderer := ui.NewRenderer()

			dev, err := mgr.Get(ctx, args[0])
			if err != nil {
				return fmt.Errorf("device not found: %w", err)
			}

			renderer.StartSpinner("Deleting %s...", dev.Name)
			if err := mgr.Delete(ctx, dev); err != nil {
				renderer.StopSpinner(false)
				return fmt.Errorf("failed to delete: %w", err)
			}

			renderer.StopSpinner(true)
			renderer.Success("Deleted %s", dev.Name)
			return nil
		},
	}
}

func devicesTypesCmd() *cobra.Command {
	var platform string

	cmd := &cobra.Command{
		Use:   "types",
		Short: "List available device types",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			mgr := device.NewManager()

			types, err := mgr.ListDeviceTypes(ctx)
			if err != nil {
				return err
			}

			for _, t := range types {
				if platform != "" && string(t.Platform) != platform {
					continue
				}
				fmt.Printf("%-40s %s\n", t.Name, t.Identifier)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&platform, "platform", "p", "", "Filter by platform")
	return cmd
}

func devicesRuntimesCmd() *cobra.Command {
	var platform string

	cmd := &cobra.Command{
		Use:   "runtimes",
		Short: "List available runtimes",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			mgr := device.NewManager()

			runtimes, err := mgr.ListRuntimes(ctx)
			if err != nil {
				return err
			}

			for _, r := range runtimes {
				if platform != "" && string(r.Platform) != platform {
					continue
				}
				status := ""
				if !r.IsAvailable {
					status = " (unavailable)"
				}
				fmt.Printf("%-20s %s%s\n", r.Name, r.Identifier, status)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&platform, "platform", "p", "", "Filter by platform (ios, watchos, tvos, visionos)")

	return cmd
}

// resolveDeviceType converts a friendly name to a CoreSimulator identifier.
func resolveDeviceType(ctx context.Context, mgr *device.Manager, input string) (string, error) {
	if strings.HasPrefix(input, "com.apple.") {
		return input, nil
	}

	types, err := mgr.ListDeviceTypes(ctx)
	if err != nil {
		return "", err
	}

	input = strings.ToLower(input)
	for _, t := range types {
		if strings.ToLower(t.Name) == input || strings.Contains(strings.ToLower(t.Name), input) {
			return t.Identifier, nil
		}
	}
	return "", fmt.Errorf("device type not found: %s", input)
}

// resolveRuntime converts a friendly name to a CoreSimulator identifier.
func resolveRuntime(ctx context.Context, mgr *device.Manager, input string) (string, error) {
	if strings.HasPrefix(input, "com.apple.") {
		return input, nil
	}

	runtimes, err := mgr.ListRuntimes(ctx)
	if err != nil {
		return "", err
	}

	input = strings.ToLower(input)
	for _, r := range runtimes {
		if !r.IsAvailable {
			continue
		}
		if strings.ToLower(r.Name) == input || strings.Contains(strings.ToLower(r.Name), input) {
			return r.Identifier, nil
		}
	}
	return "", fmt.Errorf("runtime not found: %s", input)
}
