# Support Envoy-based Node-Local Load Balancing on Windows

## Summary

This PR introduces documentation and example configurations to enable Envoy-based node-local load balancing for Windows worker nodes in k0s clusters. Windows nodes can now participate in high-availability (HA) control plane setups using a local Envoy proxy, providing the same reliability and load balancing capabilities as Linux nodes.

## Background / Motivation

Currently, k0s supports Envoy-based node-local load balancing only on Linux nodes. This creates several problems for Windows workers in HA configurations:

- **Single Point of Failure**: Windows nodes must connect directly to a single API server endpoint
- **Inconsistent Behavior**: Mixed Linux/Windows clusters handle control plane traffic differently across node types
- **Complex External LB Requirements**: Users must configure and maintain external load balancers specifically for Windows nodes

A proof-of-concept (originally documented in issue #2878) demonstrated that the existing Linux Envoy manifests can be reused on Windows with minimal changes, using:
- Envoy Windows binaries from `envoyproxy/envoy-windows-ltsc2022:v1.25.2`
- Standard Windows service management via `nssm`
- Identical `envoy.yaml` and `cds.yaml` configurations as Linux

## Changes in This PR

### 1. Documentation (`docs/windows-envoy-lb.md`)

Added comprehensive guide covering:
- Prerequisites and system requirements
- Step-by-step Envoy installation on Windows
- Service configuration and management
- Worker node join/bootstrap integration
- Current limitations and future roadmap

### 2. Example Configurations (`docs/examples/windows-envoy/`)

#### `envoy.yaml`
Bootstrap configuration for Envoy proxy:
- Listens on `127.0.0.1:7443` for API server traffic
- Listens on `127.0.0.1:8132` for Konnectivity traffic
- Uses dynamic CDS configuration from `cds.yaml`

#### `cds.yaml`
Cluster Discovery Service configuration:
- Defines upstream API server endpoints (user must customize IPs)
- Configures load balancing policies (RANDOM for API, ROUND_ROBIN for Konnectivity)
- Includes TCP health checks

#### `install-envoy.ps1`
PowerShell script to automate:
- Directory structure creation
- Envoy binary verification
- Configuration file deployment
- NSSM service installation and configuration
- Log rotation and restart policies

### 3. Implementation Approach

**Configuration Parity**
- Reuses existing Linux `envoy.yaml` and `cds.yaml` templates without modification
- Only filesystem paths differ (`C:\etc\envoy\` vs `/etc/envoy/`)

**Service Management**
- Uses NSSM (Non-Sucking Service Manager) to run Envoy as a Windows service
- Configures automatic restart on failure
- Implements log rotation (1MB file size limit)
- Ensures automatic start at boot

**Bootstrap Integration**
- Documents pattern for modifying bootstrap kubeconfig
- Rewrites API server endpoint to `https://localhost:7443`
- Supports both automated and manual join workflows

## Testing

### Test Environment
- Windows Server 2019/2022 (windows/amd64)
- k0s HA cluster with multiple controllers
- Envoy from `envoyproxy/envoy-windows-ltsc2022:v1.25.2`
- NSSM installed via Chocolatey

### Verified Scenarios

✅ **Service Management**
- Envoy successfully runs as a Windows service
- Service restarts automatically on failure
- Logs written to `C:\var\log\envoy\` with rotation
- Service starts automatically at boot

✅ **Cluster Integration**
- Windows worker joins HA k0s cluster using `https://localhost:7443`
- API traffic routed through local Envoy proxy
- Node-local load balancing operates correctly
- Control plane connectivity stable across controller failover

✅ **Configuration**
- Linux-generated `envoy.yaml` and `cds.yaml` work without modification
- Dynamic cluster updates function properly
- Health checks detect upstream failures

## How to Use

### Quick Start

1. **Install prerequisites on Windows node:**
   ```powershell
   choco install nssm -y
   ```

2. **Extract Envoy binary:**
   ```bash
   crane export --platform windows/amd64 docker.io/envoyproxy/envoy-windows-ltsc2022:v1.25.2 envoy.tar
   tar xf envoy.tar
   ```

3. **Run installation script:**
   ```powershell
   cd docs/examples/windows-envoy
   # Edit cds.yaml with your controller IPs first!
   .\install-envoy.ps1
   ```

4. **Modify bootstrap kubeconfig:**
   - Replace `server:` value with `https://localhost:7443`

5. **Join the cluster:**
   ```powershell
   k0s.exe install worker --token-file modified-kubeconfig.yaml
   k0s.exe start
   ```

### Manual Installation

See `docs/windows-envoy-lb.md` for detailed manual installation steps.

## Current Limitations

This PR provides **documentation and proof-of-concept patterns**, not full automation:

- **Manual Installation**: Envoy setup requires manual steps or custom scripts
- **Static Configuration**: `cds.yaml` requires manual updates if controller IPs change
- **No Installer Integration**: Not yet integrated with `k0s install` or `k0s windows worker` commands
- **Manual Bootstrap**: Kubeconfig modification must be done manually

These limitations are intentional for this initial release and will be addressed in follow-up work.

## Future Work

### Phase 1: Automation (Next)
- Integrate with `k0s windows worker` command
- Auto-generate `cds.yaml` from controller list
- Provide official PowerShell module for installation

### Phase 2: Packaging
- Distribute pre-packaged Envoy Windows binaries
- Version-align Envoy images with k0s releases
- Create Windows-specific installation packages

### Phase 3: Hardening
- Add health check endpoints for monitoring
- Implement automated certificate rotation
- Add CI/E2E tests for mixed Linux/Windows HA clusters
- Security hardening recommendations

## Breaking Changes

None. This PR is purely additive (documentation and examples).

## Related Issues

- Closes #2878

## Checklist

- [x] Documentation added
- [x] Example configurations provided
- [x] Installation script included
- [x] Tested on Windows Server 2019/2022
- [x] Verified with HA cluster
- [ ] CI tests (N/A - documentation only)
- [ ] Changelog updated (if required)

## How to Review

1. **Review documentation**: Check `docs/windows-envoy-lb.md` for clarity and completeness
2. **Verify examples**: Ensure `docs/examples/windows-envoy/` configs are correct
3. **Test installation script**: Run `install-envoy.ps1` in a Windows environment (optional)
4. **Check configuration parity**: Compare with Linux Envoy setup in `pkg/component/worker/nllb/`

## Additional Notes

This work was inspired by the community proof-of-concept in issue #2878 by @doctorpangloss. Special thanks for demonstrating the feasibility of this approach.

The documentation prioritizes clarity and safety, with prominent warnings about:
- Editing `cds.yaml` with actual controller IPs
- Certificate considerations for `localhost` connections
- Current lack of automation

---

**For Reviewers**: This is a documentation-focused PR. The actual code implementation for automated Windows support will follow in subsequent PRs once this baseline documentation is approved and validated by the community.
