# BugX CLI

BugX CLI is a command-line tool for managing Kubernetes service tunnels through port-forwarding. It provides a simple and intuitive interface to create, manage, and monitor port-forward connections to Kubernetes services.

## Features

- 🔌 **Port Forwarding**: Create port-forward tunnels to Kubernetes services
- 📋 **Service Discovery**: List and explore Kubernetes services
- 🔄 **Connection Management**: Manage multiple active connections
- 🚀 **Background Mode**: Run port-forwards in the background as daemon processes
- 💾 **Persistent Storage**: Track connections across sessions
- 🎯 **Smart Defaults**: Automatic port selection and pod discovery

## Installation

### Prerequisites

- Go 1.16 or later
- Kubernetes cluster access (kubeconfig)

### Build from Source

```bash
git clone <repository-url>
cd bugxcli
go build -o bugx ./cmd/bugx
```

### Install Globally

```bash
go install ./cmd/bugx
```

## Configuration

BugX CLI stores configuration in `~/.bugx/` directory:

- `config.json`: General configuration (cluster name)
- `connections.json`: Active port-forward connections

### Environment Variables

- `KUBECONFIG`: Path to kubeconfig file (default: `~/.kube/config`)

## Usage

### Service Management

#### List Services

List all services in a namespace:

```bash
bugx services list --namespace <namespace>
```

Or use the default namespace:

```bash
bugx services list
```

Options:
- `--kubeconfig, -k`: Path to kubeconfig file
- `--namespace, -n`: Namespace to list services from (default: `default`)

### Port Forwarding

#### Connect to a Service

Create a port-forward tunnel to a Kubernetes service:

```bash
bugx connect <service-name> [flags]
```

**Basic Example:**

```bash
bugx connect mysql-service --namespace production
```

**Advanced Example:**

```bash
bugx connect mysql-service \
  --namespace production \
  --localport 3307 \
  --remoteport 3306 \
  --kubeconfig ~/.kube/config
```

**Flags:**
- `--kubeconfig, -k`: Path to kubeconfig file (defaults to `KUBECONFIG` env var or `~/.kube/config`)
- `--namespace, -n`: Namespace of the service (default: `default`)
- `--localport, -l`: Local port to forward to (defaults to remote port + 1)
- `--remoteport, -r`: Remote port on the pod (defaults to first service port)
- `--background, -b`: Run port-forward in background (default: `true`)

**How it works:**
1. Finds the service in the specified namespace
2. Discovers pods behind the service using service selectors
3. Selects the first available pod
4. Creates a port-forward connection
5. Runs in background by default (or foreground if `--background=false`)

#### List Active Connections

View all active port-forward connections:

```bash
bugx connect list
```

This command shows:
- Service name and namespace
- Pod name
- Local and remote ports
- Process ID (PID)
- Connection status

#### Disconnect

Stop an active port-forward connection:

```bash
bugx disconnect <service-name> [flags]
```

**Example:**

```bash
bugx disconnect mysql-service --namespace production
```

**Flags:**
- `--namespace, -n`: Namespace of the service (default: `default`)

The command will:
1. Find the connection by service name and namespace
2. Terminate the background process (SIGTERM, then SIGKILL if needed)
3. Remove the connection from the active connections list

## Examples

### Complete Workflow

```bash
# 1. List available services
bugx services list --namespace production

# 2. Connect to a database service
bugx connect mysql-db --namespace production --localport 3307

# 3. Check active connections
bugx connect list

# 4. Disconnect when done
bugx disconnect mysql-db --namespace production
```

### Foreground Port-Forward

To run a port-forward in the foreground (useful for debugging):

```bash
bugx connect redis-service --namespace default --background=false
```

Press `Ctrl+C` to stop the connection.

### Custom Ports

```bash
# Forward local port 5432 to remote port 5432
bugx connect postgres-service \
  --namespace production \
  --localport 5432 \
  --remoteport 5432
```

## Architecture

### Project Structure

```
bugxcli/
├── cmd/
│   └── bugx/
│       ├── cmd/
│       │   ├── root.go          # Root command and CLI setup
│       │   ├── connect.go       # Port-forward connection management
│       │   ├── disconnect.go    # Disconnect connections
│       │   ├── services.go      # Service listing
│       │   ├── daemon.go        # Daemon command (internal)
│       │   ├── connection.go    # Connection state management
│       │   ├── kubeconfig.go    # Kubeconfig path resolution
│       │   └── portforward_daemon.go  # Background port-forward implementation
│       ├── config/
│       │   └── config.go        # Configuration management
│       └── main.go              # Entry point
```

### Key Components

1. **Configuration Management** (`config/config.go`):
   - Cluster name persistence
   - Secure file permissions

2. **Connection Management** (`cmd/connection.go`):
   - Track active port-forward connections
   - Process ID tracking
   - Connection state persistence
   - Thread-safe operations

3. **Port Forwarding** (`cmd/connect.go`, `cmd/portforward_daemon.go`):
   - Kubernetes client integration
   - Service and pod discovery
   - Background daemon process management
   - Foreground port-forward support

## Dependencies

- [Cobra](https://github.com/spf13/cobra): CLI framework
- [Kubernetes Client-Go](https://github.com/kubernetes/client-go): Kubernetes API client

## Security Considerations

- Configuration files use secure permissions (0600)
- Background processes run with proper signal handling

## Troubleshooting

### Connection Already Exists

If you see "connection already exists", use `bugx connect list` to see active connections and disconnect the existing one first.

### Daemon Process Fails to Start

If background port-forwards fail to start, try:
1. Running in foreground mode first to see error messages: `--background=false`
2. Check kubeconfig permissions and validity
3. Verify service and pod exist in the namespace

### Kubeconfig Not Found

Ensure:
- `KUBECONFIG` environment variable is set, or
- `~/.kube/config` exists, or
- Use `--kubeconfig` flag to specify path

### Port Already in Use

If the local port is already in use:
- Use `--localport` to specify a different port
- Check existing connections with `bugx connect list`
- Disconnect conflicting connections

## Contributing

Contributions are welcome! Please ensure:
- Code follows Go best practices
- Tests are included for new features
- Documentation is updated

## License

[Specify your license here]

## Support

For issues and questions, please open an issue on the project repository.

