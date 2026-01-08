package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// runPortForwardDaemon runs a port-forward as a daemon process
// This is called when the process is spawned in the background
func runPortForwardDaemon(config *rest.Config, namespace, podName, localPort string, remotePort int32, serviceName string) error {
	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})
	errChan := make(chan error, 1)

	// Run port-forward in goroutine
	go func() {
		err := runPortForwardInGoroutineDaemon(config, namespace, podName, localPort, remotePort, stopChan, readyChan)
		if err != nil {
			errChan <- err
		}
	}()

	// Wait for ready
	select {
	case <-readyChan:
		// Port-forward is ready
		pid := os.Getpid()
		fmt.Fprintf(os.Stderr, "Port-forward daemon started (PID: %d)\n", pid)

		// Set up signal handlers
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

		// Keep running until signal
		select {
		case err := <-errChan:
			if err != nil {
				fmt.Fprintf(os.Stderr, "Port-forward error: %v\n", err)
				updateConnectionStatus(serviceName, namespace, "stopped")
				return err
			}
		case <-sigChan:
			fmt.Fprintf(os.Stderr, "Port-forward daemon stopping...\n")
			close(stopChan)
			removeConnection(serviceName, namespace)
			return nil
		}
	case err := <-errChan:
		return fmt.Errorf("port-forward failed to start: %v", err)
	case <-time.After(10 * time.Second):
		return fmt.Errorf("port-forward timed out waiting for ready")
	}

	return nil
}

// runPortForwardInGoroutineDaemon runs port-forward in a goroutine (daemon version)
func runPortForwardInGoroutineDaemon(config *rest.Config, namespace, podName, localPort string, remotePort int32, stopChan chan struct{}, readyChan chan struct{}) error {
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
