package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

// NewDaemonCmd creates the daemon command (internal, used for background processes)
func NewDaemonCmd() *cobra.Command {
	daemonCmd := &cobra.Command{
		Use:    "daemon",
		Hidden: true, // Hide from help
		Short:  "Internal daemon command",
		Long:   `Internal command used for running background port-forwards.`,
	}

	daemonCmd.AddCommand(NewDaemonPortForwardCmd())

	return daemonCmd
}

// NewDaemonPortForwardCmd creates the daemon portforward command
func NewDaemonPortForwardCmd() *cobra.Command {
	var (
		kubeconfig string
		namespace  string
		service    string
		pod        string
		localPort  string
		remotePort string
	)

	cmd := &cobra.Command{
		Use:   "portforward",
		Short: "Run port-forward as daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if kubeconfig == "" || service == "" || pod == "" || localPort == "" || remotePort == "" {
				return fmt.Errorf("missing required flags: kubeconfig=%s, service=%s, pod=%s, localport=%s, remoteport=%s",
					kubeconfig, service, pod, localPort, remotePort)
			}

			// Build config
			config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				return fmt.Errorf("failed to build config: %v", err)
			}

			// Parse remote port
			remotePortInt, err := strconv.ParseInt(remotePort, 10, 32)
			if err != nil {
				return fmt.Errorf("invalid remote port: %v", err)
			}

			// Run daemon
			return runPortForwardDaemon(config, namespace, pod, localPort, int32(remotePortInt), service)
		},
	}

	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig")
	cmd.Flags().StringVar(&namespace, "namespace", "default", "Namespace")
	cmd.Flags().StringVar(&service, "service", "", "Service name")
	cmd.Flags().StringVar(&pod, "pod", "", "Pod name")
	cmd.Flags().StringVar(&localPort, "localport", "", "Local port")
	cmd.Flags().StringVar(&remotePort, "remoteport", "", "Remote port")

	return cmd
}
