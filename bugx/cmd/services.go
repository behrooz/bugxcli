package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewServicesCmd creates the services command
func NewServicesCmd() *cobra.Command {
	servicesCmd := &cobra.Command{
		Use:   "services",
		Short: "Manage Kubernetes services",
		Long:  `List and manage Kubernetes services in a cluster.`,
	}

	servicesCmd.AddCommand(NewServicesListCmd())

	return servicesCmd
}

// NewServicesListCmd creates the services list command
func NewServicesListCmd() *cobra.Command {
	var (
		kubeconfig string
		namespace  string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all services in a namespace",
		Long:  `List all Kubernetes services in the specified namespace.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get kubeconfig path
			kubeconfigPath := getKubeconfigPath(kubeconfig)
			if kubeconfigPath == "" {
				return fmt.Errorf("kubeconfig not found. Use --kubeconfig flag or set KUBECONFIG env var")
			}

			// Build config from kubeconfig
			config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				return fmt.Errorf("failed to build config: %v", err)
			}

			// Create clientset
			clientset, err := kubernetes.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to create clientset: %v", err)
			}

			// List services
			services, err := listServices(clientset, namespace)
			if err != nil {
				return fmt.Errorf("failed to connect to Kubernetes cluster: %v\n\nMake sure your cluster is running and accessible. Check your kubeconfig with: kubectl cluster-info", err)
			}

			// Display services
			displayServices(services, namespace)

			return nil
		},
	}

	cmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "Path to kubeconfig file (defaults to KUBECONFIG env var or ~/.kube/config)")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace to list services from")

	return cmd
}


// listServices lists all services in a namespace
func listServices(clientset *kubernetes.Clientset, namespace string) ([]ServiceInfo, error) {
	services, err := clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var serviceList []ServiceInfo
	for _, svc := range services.Items {
		var ports []string
		for _, port := range svc.Spec.Ports {
			portStr := fmt.Sprintf("%d/%s", port.Port, port.Protocol)
			if port.NodePort > 0 {
				portStr += fmt.Sprintf(" (NodePort: %d)", port.NodePort)
			}
			ports = append(ports, portStr)
		}

		serviceList = append(serviceList, ServiceInfo{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Type:      string(svc.Spec.Type),
			Ports:     ports,
			Selector:  formatSelector(svc.Spec.Selector),
		})
	}

	return serviceList, nil
}

// ServiceInfo represents service information
type ServiceInfo struct {
	Name      string
	Namespace string
	Type      string
	Ports     []string
	Selector  string
}

// formatSelector formats the service selector as a string
func formatSelector(selector map[string]string) string {
	if len(selector) == 0 {
		return "<none>"
	}

	var parts []string
	for k, v := range selector {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ",")
}

// displayServices displays services in a user-friendly format
func displayServices(services []ServiceInfo, namespace string) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Services in namespace: %s (%d total)\n", namespace, len(services))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	if len(services) == 0 {
		fmt.Println("  No services found.")
		fmt.Println()
		return
	}

	for i, svc := range services {
		fmt.Printf("  [%d] %s\n", i+1, svc.Name)
		fmt.Printf("      Type:     %s\n", svc.Type)
		fmt.Printf("      Ports:     %s\n", strings.Join(svc.Ports, ", "))
		fmt.Printf("      Selector:  %s\n", svc.Selector)
		if i < len(services)-1 {
			fmt.Println()
		}
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

