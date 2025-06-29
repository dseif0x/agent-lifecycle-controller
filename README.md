# Agent Lifecycle Controller

A simple HTTP service for remotely managing Kubernetes node power states via Wake-on-LAN and SSH shutdown commands.

## Features

- **Wake Node**: Send Wake-on-LAN packets to wake up sleeping nodes
- **Shutdown Node**: Remotely shutdown nodes via SSH
- **Web Interface**: Simple control panel for managing node states
- **Kubernetes Ready**: Includes Helm chart for easy deployment

## Prerequisites

- Go 1.24.3+
- `wakeonlan` command-line tool installed
- SSH access to target nodes
- Node with Wake-on-LAN enabled

## Configuration

Set the following environment variables or create a `.env` file:

```bash
# Wake-on-LAN configuration
NODE_MAC=AA:BB:CC:DD:EE:FF    # MAC address of the target node
WAKE_HOST=192.168.1.255       # Broadcast address for WOL packets

# SSH shutdown configuration
NODE_HOST=192.168.1.100       # IP address of the target node
NODE_USER=username            # SSH username
SSH_KEY_PATH=/path/to/key     # Path to SSH private key
```

## Usage

### Local Development

1. Install dependencies:
   ```bash
   go mod download
   ```

2. Run the service:
   ```bash
   go run main.go
   ```

3. Access the web interface at `http://localhost:8080`

### Kubernetes Deployment

#### Deploy using Helm:

1. Configure your node groups in a new file `values.yaml`:

```yaml
gpuNode:
   mac: AA:BB:CC:DD:EE:FF    # MAC address of the target node
   host: 192.168.1.100       # IP address of the target node
   wake_host: 192.168.1.255  # Broadcast address for WOL packets
   uname: username           # SSH username

```
2. Deploy with Helm:
```bash
helm repo add agent-lifecycle-controller https://dseif0x.github.io/agent-lifecycle-controller
helm install agent-lifecycle-controller --namespace agent-lifecycle-controller --create-namespace --values values.yaml agent-lifecycle-controller/agent-lifecycle-controller
```

This will require a secret named `visus-ssh-key` with the SSH key of the node in the `id_rsa` field to to exist in the same namespace.

## API Endpoints

- `POST /wake` - Send Wake-on-LAN packet to wake the node
- `POST /shutdown` - Shutdown the node via SSH
- `GET /` - Web interface

## Docker

Build the Docker image:

```bash
docker build -t agent-lifecycle-controller .
```

## Security Notes

- Uses SSH key-based authentication
- Requires proper network access to target nodes
- SSH host key verification is disabled (insecure for production)