package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// NewConnectCmd creates the connect command
func NewConnectCmd() *cobra.Command {
	var (
		kubeconfig string
		namespace  string
		localPort  string
		remotePort string
		background bool
	)

	cmd := &cobra.Command{
		Use:   "connect [servicename]",
		Short: "Create a port-forward tunnel to a service",
		Long: `Create a port-forward tunnel to expose a Kubernetes service locally.
		
This command finds a pod behind the service and creates a port-forward connection.
Use --background to run in the background (default).`,
		Args: cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no args, show help or list
			if len(args) == 0 {
				return cmd.Help()
			}

			servicename := args[0]

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

			// Default namespace
			if namespace == "" {
				namespace = "default"
			}

			// Get service to find selector and port
			svc, err := clientset.CoreV1().Services(namespace).Get(context.TODO(), servicename, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get service: %v", err)
			}

			// Determine remote port
			remotePortInt := int32(3306) // Default
			if remotePort != "" {
				port, err := strconv.ParseInt(remotePort, 10, 32)
				if err != nil {
					return fmt.Errorf("invalid remote port: %v", err)
				}
				remotePortInt = int32(port)
			} else if len(svc.Spec.Ports) > 0 {
				remotePortInt = svc.Spec.Ports[0].Port
			}

			// Find a pod behind the service
			var selectorParts []string
			for k, v := range svc.Spec.Selector {
				selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
			}
			selector := strings.Join(selectorParts, ",")

			if selector == "" {
				return fmt.Errorf("service %s has no selector", servicename)
			}

			pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
				LabelSelector: selector,
			})
			if err != nil {
				return fmt.Errorf("failed to list pods: %v", err)
			}

			if len(pods.Items) == 0 {
				return fmt.Errorf("no pods found for service %s with selector %s", servicename, selector)
			}

			podName := pods.Items[0].Name

			// Determine local port
			localPortInt := "3307" // Default
			if localPort != "" {
				localPortInt = localPort
			} else {
				localPortInt = strconv.Itoa(int(remotePortInt) + 1)
			}

			// Check if connection already exists
			existing, _ := findConnection(servicename, namespace)
			if existing != nil && existing.Status == "active" {
				return fmt.Errorf("connection to %s/%s already exists on localhost:%s", namespace, servicename, existing.LocalPort)
			}

			if background {
				// Run in background
				return createBackgroundPortForward(config, clientset, namespace, servicename, podName, localPortInt, remotePortInt, kubeconfigPath)
			} else {
				// Run in foreground
				return createForegroundPortForward(config, clientset, namespace, podName, localPortInt, remotePortInt)
			}
		},
	}

	cmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "Path to kubeconfig file (defaults to KUBECONFIG env var or ~/.kube/config)")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace of the service")
	cmd.Flags().StringVarP(&localPort, "localport", "l", "", "Local port to forward to (defaults to remote port + 1)")
	cmd.Flags().StringVarP(&remotePort, "remoteport", "r", "", "Remote port on the pod (defaults to first service port)")
	cmd.Flags().BoolVarP(&background, "background", "b", true, "Run port-forward in background")

	// Add list as a subcommand
	cmd.AddCommand(NewConnectListCmd())

	return cmd
}

// NewConnectListCmd creates the connect list command
func NewConnectListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all active port-forward connections",
		Long:  `List all active port-forward connections.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			connections, err := loadConnections()
			if err != nil {
				return fmt.Errorf("failed to load connections: %v", err)
			}

			// Filter active connections
			var activeConnections []ConnectionInfo
			for _, conn := range connections {
				// Check if process is still running
				if isProcessRunning(conn.PID) {
					activeConnections = append(activeConnections, conn)
				} else {
					// Update status to stopped
					updateConnectionStatus(conn.ServiceName, conn.Namespace, "stopped")
				}
			}

			displayConnections(activeConnections)
			return nil
		},
	}

	return cmd
}

// createForegroundPortForward creates a port-forward connection in foreground
func createForegroundPortForward(config *rest.Config, clientset *kubernetes.Clientset, namespace, podName, localPort string, remotePort int32) error {
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return fmt.Errorf("failed to create round tripper: %v", err)
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	hostIP := strings.TrimPrefix(config.Host, "https://")
	hostIP = strings.TrimPrefix(hostIP, "http://")

	serverURL := &url.URL{
		Scheme: "https",
		Path:   path,
		Host:   hostIP,
	}

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, serverURL)

	ports := []string{fmt.Sprintf("%s:%d", localPort, remotePort)}
	pf, err := portforward.New(dialer, ports, stopChan, readyChan, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create port-forward: %v", err)
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- pf.ForwardPorts()
	}()

	select {
	case <-readyChan:
		fmt.Println()
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("  Port-forward established successfully!\n")
		fmt.Printf("  Pod:     %s/%s\n", namespace, podName)
		fmt.Printf("  Local:   localhost:%s\n", localPort)
		fmt.Printf("  Remote:  %d\n", remotePort)
		fmt.Println()
		fmt.Println("  Press Ctrl+C to stop the port-forward")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("port-forward failed: %v", err)
		}
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	fmt.Println("\nStopping port-forward...")
	close(stopChan)
	<-errChan
	fmt.Println("Port-forward stopped.")
	return nil
}

// createBackgroundPortForward creates a port-forward connection in background by spawning a daemon process
func createBackgroundPortForward(config *rest.Config, clientset *kubernetes.Clientset, namespace, serviceName, podName, localPort string, remotePort int32, kubeconfigPath string) error {
	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Use nohup or direct exec with proper daemonization
	// Create command to run daemon
	cmd := exec.Command(execPath, "daemon", "portforward",
		"--kubeconfig", kubeconfigPath,
		"--namespace", namespace,
		"--service", serviceName,
		"--pod", podName,
		"--localport", localPort,
		"--remoteport", strconv.Itoa(int(remotePort)),
	)

	// Set up process group to detach from parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session (daemon)
	}

	// Redirect stdin/stdout/stderr to /dev/null or log file
	nullFile, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
	if err != nil {
		// Fallback to Discard if /dev/null not available
		cmd.Stdin = nil
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	} else {
		cmd.Stdin = nullFile
		cmd.Stdout = nullFile
		cmd.Stderr = nullFile
		defer nullFile.Close()
	}

	// Start the daemon process (don't wait for it)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon process: %v", err)
	}

	pid := cmd.Process.Pid

	// Don't wait for the process - let it run independently
	// Release the process handle so parent can exit
	_ = cmd.Process.Release()

	// Give it a moment to start and initialize
	time.Sleep(800 * time.Millisecond)

	// Check if process is still running (give it more time)
	time.Sleep(200 * time.Millisecond)
	if !isProcessRunning(pid) {
		// Daemon failed to start - return error instead of falling back
		return fmt.Errorf("daemon process (PID %d) failed to start or exited immediately. The daemon may have encountered an error. Try running the daemon manually to see the error: bugx daemon portforward --kubeconfig %s --namespace %s --service %s --pod %s --localport %s --remoteport %d",
			pid, kubeconfigPath, namespace, serviceName, podName, localPort, remotePort)
	}

	// Save connection info
	conn := ConnectionInfo{
		PID:         pid,
		ServiceName: serviceName,
		Namespace:   namespace,
		LocalPort:   localPort,
		RemotePort:  remotePort,
		PodName:     podName,
		Kubeconfig:  kubeconfigPath,
		Status:      "active",
	}

	if err := addConnection(conn); err != nil {
		// Try to kill the process if we can't save connection info
		if proc, err := os.FindProcess(pid); err == nil {
			proc.Kill()
		}
		return fmt.Errorf("failed to save connection info: %v", err)
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Port-forward started in background!\n")
	fmt.Printf("  Service: %s/%s\n", namespace, serviceName)
	fmt.Printf("  Pod:     %s\n", podName)
	fmt.Printf("  Local:   localhost:%s\n", localPort)
	fmt.Printf("  Remote:  %d\n", remotePort)
	fmt.Printf("  PID:     %d\n", pid)
	fmt.Println()
	fmt.Printf("  Use 'bugx connect list' to see all connections\n")
	fmt.Printf("  Use 'bugx disconnect %s' to stop this connection\n", serviceName)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	return nil
}

// createBackgroundPortForwardInProcess creates port-forward in current process (fallback)
func createBackgroundPortForwardInProcess(config *rest.Config, namespace, serviceName, podName, localPort string, remotePort int32, kubeconfigPath string) error {
	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})
	errChan := make(chan error, 1)

	go func() {
		err := runPortForwardInGoroutine(config, namespace, podName, localPort, remotePort, stopChan, readyChan)
		if err != nil {
			errChan <- err
		}
	}()

	// Wait for ready with timeout
	select {
	case <-readyChan:
		pid := os.Getpid()
		conn := ConnectionInfo{
			PID:         pid,
			ServiceName: serviceName,
			Namespace:   namespace,
			LocalPort:   localPort,
			RemotePort:  remotePort,
			PodName:     podName,
			Kubeconfig:  kubeconfigPath,
			Status:      "active",
		}

		if err := addConnection(conn); err != nil {
			close(stopChan)
			return fmt.Errorf("failed to save connection info: %v", err)
		}

		fmt.Println()
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("  Port-forward started in background!\n")
		fmt.Printf("  Service: %s/%s\n", namespace, serviceName)
		fmt.Printf("  Pod:     %s\n", podName)
		fmt.Printf("  Local:   localhost:%s\n", localPort)
		fmt.Printf("  Remote:  %d\n", remotePort)
		fmt.Printf("  PID:     %d\n", pid)
		fmt.Println()
		fmt.Printf("  Note: Process will keep running in background.\n")
		fmt.Printf("  Use 'bugx connect list' to see all connections\n")
		fmt.Printf("  Use 'bugx disconnect %s' to stop this connection\n", serviceName)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()

		// Set up signal handler to clean up on exit
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Keep process alive and handle errors
		go func() {
			select {
			case err := <-errChan:
				if err != nil {
					fmt.Fprintf(os.Stderr, "Port-forward error: %v\n", err)
					updateConnectionStatus(serviceName, namespace, "stopped")
				}
			case <-sigChan:
				close(stopChan)
				removeConnection(serviceName, namespace)
				os.Exit(0)
			}
		}()

		// Block forever to keep process alive
		select {}
	case err := <-errChan:
		return fmt.Errorf("port-forward failed: %v", err)
	}
}

// runPortForwardInGoroutine runs port-forward in a goroutine
func runPortForwardInGoroutine(config *rest.Config, namespace, podName, localPort string, remotePort int32, stopChan chan struct{}, readyChan chan struct{}) error {
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return fmt.Errorf("failed to create round tripper: %v", err)
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	hostIP := strings.TrimPrefix(config.Host, "https://")
	hostIP = strings.TrimPrefix(hostIP, "http://")

	serverURL := &url.URL{
		Scheme: "https",
		Path:   path,
		Host:   hostIP,
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, serverURL)

	ports := []string{fmt.Sprintf("%s:%d", localPort, remotePort)}
	pf, err := portforward.New(dialer, ports, stopChan, readyChan, io.Discard, io.Discard)
	if err != nil {
		return fmt.Errorf("failed to create port-forward: %v", err)
	}

	return pf.ForwardPorts()
}

// displayConnections displays connections in a user-friendly format
func displayConnections(connections []ConnectionInfo) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if len(connections) == 0 {
		fmt.Println("  No Active Connections")
	} else {
		fmt.Printf("  Active Connections (%d)\n", len(connections))
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	for i, conn := range connections {
		fmt.Printf("  [%d] %s/%s\n", i+1, conn.Namespace, conn.ServiceName)
		fmt.Printf("      Pod:      %s\n", conn.PodName)
		fmt.Printf("      Local:    localhost:%s\n", conn.LocalPort)
		fmt.Printf("      Remote:   %d\n", conn.RemotePort)
		fmt.Printf("      PID:      %d\n", conn.PID)
		fmt.Printf("      Status:   %s\n", conn.Status)
		if i < len(connections)-1 {
			fmt.Println()
		}
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// isProcessRunning checks if a process is still running
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
