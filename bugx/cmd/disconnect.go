package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
)

// NewDisconnectCmd creates the disconnect command
func NewDisconnectCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "disconnect [servicename]",
		Short: "Disconnect a port-forward connection",
		Long:  `Disconnect an active port-forward connection by service name.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			servicename := args[0]

			if namespace == "" {
				namespace = "default"
			}

			// Find connection
			conn, err := findConnection(servicename, namespace)
			if err != nil {
				return fmt.Errorf("connection not found: %s/%s", namespace, servicename)
			}

			// Check if process is running
			if !isProcessRunning(conn.PID) {
				// Process already stopped, just remove from list
				removeConnection(servicename, namespace)
				fmt.Printf("Connection to %s/%s was already stopped.\n", namespace, servicename)
				return nil
			}

			// Kill the process
			process, err := os.FindProcess(conn.PID)
			if err != nil {
				return fmt.Errorf("failed to find process %d: %v", conn.PID, err)
			}

			if err := process.Signal(syscall.SIGTERM); err != nil {
				// Try SIGKILL if SIGTERM fails
				if err := process.Signal(syscall.SIGKILL); err != nil {
					return fmt.Errorf("failed to kill process %d: %v", conn.PID, err)
				}
			}

			// Remove from connections list
			if err := removeConnection(servicename, namespace); err != nil {
				return fmt.Errorf("failed to remove connection: %v", err)
			}

			fmt.Println()
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Printf("  Connection Disconnected\n")
			fmt.Printf("  Service: %s/%s\n", namespace, servicename)
			fmt.Printf("  PID:     %d\n", conn.PID)
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace of the service")

	return cmd
}

