package cli

import (
	"context"

	"github.com/arnavsurve/swiftctl/internal/process"
	"github.com/spf13/cobra"
)

var (
	verbose bool
	rootCmd *cobra.Command
)

func init() {
	rootCmd = &cobra.Command{
		Use:   "swiftctl",
		Short: "Terminal-first Swift development",
		Long: `swiftctl provides a unified CLI for building, running, and debugging Swift apps.

Common workflows:
  swiftctl run ios          Build, deploy, launch, and stream logs
  swiftctl run ios -w       Same, but rebuild on file changes
  swiftctl devices list     Show available simulators
  swiftctl build            Just build the project`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			process.SetGlobalVerbose(verbose)
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show underlying commands")
}

func Execute(ctx context.Context, version string) error {
	rootCmd.Version = version

	rootCmd.AddCommand(devicesCmd())
	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(projectCmd())
	rootCmd.AddCommand(runCmd())

	return rootCmd.ExecuteContext(ctx)
}

func Verbose() bool { return verbose }
