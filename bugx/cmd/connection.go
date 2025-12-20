package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ConnectionInfo stores information about an active port-forward connection
type ConnectionInfo struct {
	PID         int    `json:"pid"`
	ServiceName string `json:"service_name"`
	Namespace   string `json:"namespace"`
	LocalPort   string `json:"local_port"`
	RemotePort  int32  `json:"remote_port"`
	PodName     string `json:"pod_name"`
	Kubeconfig  string `json:"kubeconfig"`
	Status      string `json:"status"` // "active", "stopped"
}

var (
	connectionsMutex sync.Mutex
	connectionsFile  string
)

func init() {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".bugx")
	connectionsFile = filepath.Join(configDir, "connections.json")
}

// getConnectionsFile returns the path to the connections file
func getConnectionsFile() string {
	return connectionsFile
}

// loadConnections loads all connections from the file
func loadConnections() ([]ConnectionInfo, error) {
	filePath := getConnectionsFile()
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %v", err)
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ConnectionInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read connections file: %v", err)
	}

	var connections []ConnectionInfo
	if len(data) > 0 {
		if err := json.Unmarshal(data, &connections); err != nil {
			return nil, fmt.Errorf("failed to parse connections file: %v", err)
		}
	}

	return connections, nil
}

// saveConnections saves connections to the file
func saveConnections(connections []ConnectionInfo) error {
	filePath := getConnectionsFile()
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	data, err := json.MarshalIndent(connections, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal connections: %v", err)
	}

	return os.WriteFile(filePath, data, 0600)
}

// addConnection adds a new connection to the list
func addConnection(conn ConnectionInfo) error {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()

	connections, err := loadConnections()
	if err != nil {
		return err
	}

	connections = append(connections, conn)
	return saveConnections(connections)
}

// removeConnection removes a connection by service name
func removeConnection(serviceName, namespace string) error {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()

	connections, err := loadConnections()
	if err != nil {
		return err
	}

	var updated []ConnectionInfo
	for _, conn := range connections {
		if conn.ServiceName != serviceName || conn.Namespace != namespace {
			updated = append(updated, conn)
		}
	}

	return saveConnections(updated)
}

// updateConnectionStatus updates the status of a connection
func updateConnectionStatus(serviceName, namespace, status string) error {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()

	connections, err := loadConnections()
	if err != nil {
		return err
	}

	for i := range connections {
		if connections[i].ServiceName == serviceName && connections[i].Namespace == namespace {
			connections[i].Status = status
			return saveConnections(connections)
		}
	}

	return fmt.Errorf("connection not found")
}

// findConnection finds a connection by service name and namespace
func findConnection(serviceName, namespace string) (*ConnectionInfo, error) {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()

	connections, err := loadConnections()
	if err != nil {
		return nil, err
	}

	for _, conn := range connections {
		if conn.ServiceName == serviceName && conn.Namespace == namespace {
			return &conn, nil
		}
	}

	return nil, fmt.Errorf("connection not found")
}

