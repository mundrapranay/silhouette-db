# Deployment Strategy Analysis for Real Network Testing

This document analyzes different approaches for deploying and testing `silhouette-db` on a real network with multiple server nodes and worker nodes accessible via SSH.

## Requirements

- **Server Nodes**: Multiple nodes for Raft cluster (e.g., 3-10 nodes)
- **Worker Nodes**: Multiple nodes for LEDP workers (e.g., 10-100 workers)
- **Access**: SSH access to all nodes
- **Goals**: 
  - Simple deployment and setup
  - Easy to start/stop services
  - Manageable configuration
  - Minimal manual intervention

## Option 1: Docker-Based Deployment

### Overview

Containerize `silhouette-db` server and workers using Docker, then deploy containers across nodes using Docker Swarm, Kubernetes, or simple `docker run` commands.

### Architecture

```
┌─────────────────────────────────────────┐
│  Deployment Node (Control Machine)      │
│  - Docker images built                  │
│  - Deployment scripts                  │
│  - Configuration files                  │
└─────────────────┬───────────────────────┘
                  │ SSH/Docker
                  ▼
┌─────────────────────────────────────────┐
│  Server Nodes (Raft Cluster)            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐│
│  │ Node 1  │  │ Node 2  │  │ Node 3  ││
│  │ Docker  │◄─►│ Docker  │◄─►│ Docker  ││
│  │ Server  │  │ Server  │  │ Server  ││
│  └──────────┘  └──────────┘  └──────────┘│
└─────────────────────────────────────────┘
                  │ gRPC
                  ▼
┌─────────────────────────────────────────┐
│  Worker Nodes (LEDP Workers)            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐│
│  │Worker-0 │  │Worker-1 │  │Worker-N ││
│  │ Docker  │  │ Docker  │  │ Docker  ││
│  └──────────┘  └──────────┘  └──────────┘│
└─────────────────────────────────────────┘
```

### Implementation Approach

1. **Dockerfile for Server**:
   ```dockerfile
   FROM golang:1.24-alpine AS builder
   # Build silhouette-server binary
   
   FROM alpine:latest
   COPY --from=builder /app/silhouette-server /usr/local/bin/
   EXPOSE 8080 9090
   CMD ["silhouette-server", ...]
   ```

2. **Dockerfile for Worker**:
   ```dockerfile
   FROM golang:1.24-alpine AS builder
   # Build algorithm-runner binary
   
   FROM alpine:latest
   COPY --from=builder /app/algorithm-runner /usr/local/bin/
   CMD ["algorithm-runner", ...]
   ```

3. **Docker Compose** (for local testing):
   ```yaml
   version: '3.8'
   services:
     server-1:
       image: silhouette-server:latest
       ports: ["8080:8080", "9090:9090"]
       environment:
         - NODE_ID=node1
         - BOOTSTRAP=true
   ```

4. **Deployment Script**:
   ```bash
   # Build images
   docker build -t silhouette-server:latest -f Dockerfile.server .
   docker build -t algorithm-runner:latest -f Dockerfile.worker .
   
   # Push to registry (or copy to nodes)
   docker save silhouette-server:latest | ssh node1 docker load
   
   # Start services on nodes
   ssh node1 "docker run -d --name server silhouette-server:latest ..."
   ```

### Pros

✅ **Isolation**: Clean environment, no dependency conflicts
✅ **Portability**: Same image works across different OS/distributions
✅ **Reproducibility**: Consistent environment across nodes
✅ **Resource Management**: Easy to set CPU/memory limits
✅ **Orchestration Ready**: Can use Docker Swarm/Kubernetes later
✅ **Easy Cleanup**: `docker stop/rm` removes everything cleanly
✅ **Logging**: Built-in Docker logging (`docker logs`)
✅ **Networking**: Docker network isolation can help with testing

### Cons

❌ **Docker Dependency**: Requires Docker on all nodes
❌ **Build Time**: Need to build/transfer images
❌ **Complexity**: More moving parts (images, containers, networking)
❌ **Debugging**: Slightly harder to debug inside containers
❌ **Overhead**: Container overhead (minimal but present)
❌ **Binary Size**: Docker images can be large to transfer
❌ **Rust Dependencies**: Need to include Rust runtime for FFI libraries

### Setup Complexity: **Medium**

---

## Option 2: Script-Based Deployment (pssh/parallel-ssh)

### Overview

Use parallel SSH tools (pssh, parallel-ssh, or custom scripts) to deploy binaries and start services directly on nodes.

### Architecture

```
┌─────────────────────────────────────────┐
│  Deployment Node (Control Machine)      │
│  - Build binaries                       │
│  - pssh/parallel-ssh installed          │
│  - Deployment scripts                  │
│  - Node inventory files                 │
└─────────────────┬───────────────────────┘
                  │ SSH
                  ▼
┌─────────────────────────────────────────┐
│  Nodes (All accessible via SSH)         │
│  - Binaries deployed to /opt/silhouette │
│  - Systemd services (optional)          │
│  - Configuration in /etc/silhouette     │
└─────────────────────────────────────────┘
```

### Implementation Approach

1. **Build Binaries Locally**:
   ```bash
   make build
   make build-algorithm-runner
   ```

2. **Node Inventory Files**:
   ```
   # servers.txt
   node1:192.168.1.10
   node2:192.168.1.11
   node3:192.168.1.12
   
   # workers.txt
   worker0:192.168.1.20
   worker1:192.168.1.21
   worker2:192.168.1.22
   ```

3. **Deployment Script** (using pssh):
   ```bash
   # Copy binaries to servers
   pscp -h servers.txt -l user silhouette-server /opt/silhouette/
   
   # Copy configuration files
   pscp -h servers.txt -l user configs/node1.hcl /etc/silhouette/
   
   # Start services
   pssh -h servers.txt -l user \
     "sudo systemctl start silhouette-server"
   ```

4. **Simple Deployment Script** (custom):
   ```bash
   # deploy-servers.sh
   for node in $(cat servers.txt); do
     ssh $node "mkdir -p /opt/silhouette"
     scp bin/silhouette-server $node:/opt/silhouette/
     scp configs/node*.hcl $node:/etc/silhouette/
     ssh $node "cd /opt/silhouette && ./silhouette-server ..."
   done
   ```

### Pros

✅ **Simplicity**: Direct binary deployment, no container overhead
✅ **No Dependencies**: Only requires SSH on nodes (universal)
✅ **Fast Deployment**: Just copy binaries and run
✅ **Easy Debugging**: Direct access to processes, logs, files
✅ **Lightweight**: Minimal setup on nodes
✅ **Familiar**: Standard SSH-based deployment pattern
✅ **Flexible**: Easy to customize per node
✅ **Rust Dependencies**: Can build on nodes or include in binary path

### Cons

❌ **Environment Differences**: Need to handle different OS/architectures
❌ **Dependency Management**: Must ensure Rust/C dependencies are installed
❌ **Process Management**: Need to handle process management (systemd, supervisor, etc.)
❌ **Cleanup**: Manual cleanup of processes/logs
❌ **Error Handling**: More complex error handling across nodes
❌ **Configuration Sync**: Must keep configs synchronized

### Setup Complexity: **Low to Medium**

---

## Option 3: Hybrid Approach (Binaries + Simple Scripts)

### Overview

Combine simplicity of script-based deployment with lightweight process management using systemd or supervisor.

### Architecture

```
┌─────────────────────────────────────────┐
│  Deployment Script                      │
│  - Build binaries (one architecture)    │
│  - Copy to nodes                        │
│  - Generate systemd unit files         │
│  - Start services                       │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  Nodes with:                            │
│  - Binaries in /opt/silhouette         │
│  - systemd units in /etc/systemd/      │
│  - Configs in /etc/silhouette/         │
│  - Logs in /var/log/silhouette/        │
└─────────────────────────────────────────┘
```

### Implementation Approach

1. **Deployment Script**:
   ```bash
   #!/bin/bash
   # deploy.sh
   
   # Build binaries
   make build build-algorithm-runner
   
   # Deploy to servers
   for node in $(cat servers.txt); do
     ssh $node "sudo mkdir -p /opt/silhouette /etc/silhouette /var/log/silhouette"
     scp bin/silhouette-server $node:/opt/silhouette/
     scp configs/node*.hcl $node:/etc/silhouette/
     scp deploy/systemd/silhouette-server.service $node:/tmp/
     ssh $node "sudo mv /tmp/silhouette-server.service /etc/systemd/system/"
     ssh $node "sudo systemctl daemon-reload && sudo systemctl enable silhouette-server"
   done
   ```

2. **Systemd Unit File**:
   ```ini
   [Unit]
   Description=silhouette-db Server
   After=network.target
   
   [Service]
   Type=simple
   User=silhouette
   ExecStart=/opt/silhouette/silhouette-server \
     -node-id=node1 \
     -listen-addr=0.0.0.0:8080 \
     -grpc-addr=0.0.0.0:9090 \
     -data-dir=/var/lib/silhouette/data \
     -bootstrap=false
   Restart=always
   RestartSec=10
   
   [Install]
   WantedBy=multi-user.target
   ```

3. **Process Management**:
   ```bash
   # Start all servers
   pssh -h servers.txt "sudo systemctl start silhouette-server"
   
   # Check status
   pssh -h servers.txt "sudo systemctl status silhouette-server"
   
   # View logs
   pssh -h servers.txt "sudo journalctl -u silhouette-server -f"
   ```

### Pros

✅ **Simple Deployment**: Just copy binaries and configs
✅ **Process Management**: systemd handles restarts, logging
✅ **Standard Pattern**: Familiar Linux deployment pattern
✅ **No Container Overhead**: Direct execution
✅ **Easy Monitoring**: systemd/journalctl for logs
✅ **Auto-restart**: Handles crashes automatically
✅ **Production-Ready**: Similar to production deployments

### Cons

❌ **OS Dependency**: Requires systemd (Linux systems)
❌ **Architecture Matching**: All nodes must be same architecture
❌ **Manual Setup**: Need to create systemd files, users, directories
❌ **Configuration Management**: Must manage configs per node

### Setup Complexity: **Medium**

---

## Option 4: Ansible-Based Deployment

### Overview

Use Ansible playbooks for declarative, idempotent deployment across nodes.

### Pros

✅ **Idempotent**: Safe to run multiple times
✅ **Declarative**: Describe desired state
✅ **Inventory Management**: Built-in node management
✅ **Template Support**: Jinja2 templates for configs
✅ **Conditional Logic**: Handle different node types easily
✅ **Fault Tolerance**: Better error handling and rollback
✅ **Community**: Large ecosystem of modules

### Cons

❌ **Learning Curve**: Need to learn Ansible
❌ **Overhead**: Additional tool dependency
❌ **Complexity**: May be overkill for simple deployments
❌ **Python Required**: On control machine

### Setup Complexity: **Medium to High**

---

## Recommendation

### For Your Use Case: **Option 3 (Hybrid: Scripts + Systemd)**

**Reasoning:**

1. **Simplicity**: No Docker overhead, straightforward SSH deployment
2. **Familiar Pattern**: Standard Linux deployment approach
3. **Easy Management**: systemd handles process lifecycle
4. **Quick Setup**: Minimal infrastructure requirements
5. **Debuggable**: Direct access to processes and logs
6. **Flexible**: Easy to customize per node
7. **Production-Like**: Similar to real production deployments

### Implementation Structure

```
silhouette-db/
├── deploy/
│   ├── deploy.sh              # Main deployment script
│   ├── stop.sh                # Stop all services
│   ├── status.sh              # Check status
│   ├── logs.sh                # View logs
│   ├── cleanup.sh             # Clean up
│   ├── inventory/
│   │   ├── servers.txt        # Server node list
│   │   └── workers.txt        # Worker node list
│   ├── configs/
│   │   ├── node1.hcl          # Server configs
│   │   ├── node2.hcl
│   │   └── worker-*.yaml      # Worker configs
│   ├── systemd/
│   │   ├── silhouette-server.service
│   │   └── algorithm-runner.service
│   └── README.md              # Deployment guide
```

### Deployment Workflow

1. **Prepare** (on deployment machine):
   ```bash
   make build build-algorithm-runner
   ./deploy/deploy.sh
   ```

2. **Script Actions**:
   - Copy binaries to nodes
   - Copy configuration files
   - Install systemd units
   - Start services
   - Verify health

3. **Management**:
   ```bash
   ./deploy/status.sh      # Check all services
   ./deploy/logs.sh        # View logs
   ./deploy/stop.sh        # Stop all
   ./deploy/cleanup.sh     # Remove everything
   ```

### Alternative: If Docker is Required

If you need Docker (e.g., for containerized environments), use **Option 1** but keep it simple:

1. Single Dockerfile for both server and worker (multi-stage)
2. Simple deployment script using `docker run`
3. No orchestration complexity (avoid Swarm/Kubernetes initially)
4. Use Docker Compose for local testing only

---

## Comparison Table

| Feature | Docker | Scripts (pssh) | Hybrid (systemd) | Ansible |
|---------|--------|---------------|------------------|---------|
| **Setup Complexity** | Medium | Low | Medium | Medium-High |
| **Dependencies** | Docker on all nodes | SSH only | systemd + SSH | Ansible + Python |
| **Deployment Speed** | Slower (image transfer) | Fast (binary copy) | Fast | Medium |
| **Process Management** | Docker | Manual | systemd | Ansible |
| **Debugging** | Medium | Easy | Easy | Medium |
| **Resource Overhead** | Medium | Low | Low | Low |
| **Portability** | High | Low | Low | Medium |
| **Production Ready** | Yes | Partial | Yes | Yes |
| **Learning Curve** | Medium | Low | Low | Medium-High |

---

## Quick Start Recommendation

**Start with Option 3 (Hybrid)**:

1. Create `deploy/` directory structure
2. Write simple deployment script
3. Create systemd unit files
4. Test on 2-3 nodes first
5. Scale up gradually

**If you need containerization later**, migrate to Docker but keep deployment scripts simple (no orchestration initially).

---

## Questions to Consider

1. **Node Uniformity**: Are all nodes the same OS/architecture?
   - If yes → Script-based is easier
   - If no → Docker provides portability

2. **Network Configuration**: 
   - Do nodes need specific networking setup?
   - Docker networking may be easier to configure

3. **Process Management Requirements**:
   - Need auto-restart? → systemd or Docker
   - Just run manually? → Simple scripts work

4. **Scalability Needs**:
   - Few nodes (3-10)? → Scripts are fine
   - Many nodes (50+)? → Consider orchestration

5. **Team Familiarity**:
   - Comfortable with Docker? → Use Docker
   - Prefer simple scripts? → Use scripts

---

## Conclusion

For your use case (testing on real network with SSH access), I recommend:

**Option 3: Hybrid Script-Based Deployment with systemd**

This provides the best balance of:
- ✅ Simplicity
- ✅ Quick setup
- ✅ Easy management
- ✅ Production-like environment
- ✅ Minimal dependencies

Docker can be added later if needed, but starting simple will get you testing faster and reduce complexity.

