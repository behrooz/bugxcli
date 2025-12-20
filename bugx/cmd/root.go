package cmd

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root command
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "bugx",
		Short: "BugX CLI - Manage service tunnels",
		Long:  `BugX CLI is a command-line tool for managing creating service tunnels.`,
	}

	// Add subcommands
	rootCmd.AddCommand(NewConnectCmd())
	rootCmd.AddCommand(NewServicesCmd())
	rootCmd.AddCommand(NewDisconnectCmd())
	rootCmd.AddCommand(NewDaemonCmd())

	return rootCmd
}
