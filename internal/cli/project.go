package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/arnavsurve/swiftctl/internal/project"
	"github.com/arnavsurve/swiftctl/internal/ui"
	"github.com/spf13/cobra"
)

func projectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Project introspection",
		Long:  `Inspect and display information about the current Swift project.`,
	}

	cmd.AddCommand(projectInfoCmd())

	return cmd
}

func projectInfoCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show project information",
		Long:  `Detect and display information about the Swift project in the current directory.`,
		Example: `  swiftctl project info
  swiftctl project info --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			detector := project.NewDetector()

			info, err := detector.Detect(".")
			if err != nil {
				return fmt.Errorf("no project found: %w", err)
			}

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}

			renderer := ui.NewRenderer()
			renderer.Success("Project: %s", info.Name)
			renderer.Info("Type: %s", info.Type)
			renderer.Info("Path: %s", info.Path)

			if len(info.Schemes) > 0 {
				renderer.Info("")
				renderer.Info("Schemes:")
				for _, s := range info.Schemes {
					renderer.Info("  • %s", s)
				}
			}

			if len(info.Platforms) > 0 {
				renderer.Info("")
				renderer.Info("Platforms:")
				for _, p := range info.Platforms {
					renderer.Info("  • %s", p)
				}
			}

			if len(info.Targets) > 0 {
				renderer.Info("")
				renderer.Info("Targets:")
				for _, t := range info.Targets {
					if t.ProductType != "" {
						renderer.Info("  • %s (%s)", t.Name, t.ProductType)
					} else {
						renderer.Info("  • %s", t.Name)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}
