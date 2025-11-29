# Envoy-based Node-Local Load Balancing on Windows

This guide describes how to set up Envoy-based node-local load balancing for Windows worker nodes in a k0s cluster. This allows Windows nodes to communicate with the control plane via a local proxy, providing high availability and consistency with Linux nodes.

## Prerequisites

*   A running k0s cluster with High Availability (multiple controllers).
*   A Windows Server node.
*   `nssm` (Non-Sucking Service Manager) installed on the Windows node (e.g., `choco install nssm`).
*   Administrator access to the Windows node.

## Step 1: Install Envoy on Windows

Since Envoy does not distribute official Windows binaries separately, you must extract one from the official Docker image or use a third-party build.

### Option A: Extract from Docker Image (Recommended)

You can use `crane` to extract the binary without installing Docker Desktop on Windows.

1.  Download `crane` for Windows.
2.  Run the following commands in PowerShell:

    ```powershell
    # Export the filesystem of the envoy image
    crane export --platform windows/amd64 docker.io/envoyproxy/envoy-windows-ltsc2022:v1.25.2 envoy.tar
    
    # Extract the tarball
    tar xf envoy.tar
    
    # Create the destination directory
    New-Item -ItemType Directory -Force -Path "C:\Program Files\envoy"
    
    # Move the binary (adjust path if the image structure changes)
    Move-Item -Path "Files\Program Files\envoy\envoy.exe" -Destination "C:\Program Files\envoy\envoy.exe"
    ```

### Option B: Manual Placement

If you have the `envoy.exe` binary from another source, place it at `C:\Program Files\envoy\envoy.exe`.

## Step 2: Configure Envoy

We need to create the Envoy configuration files that mimic the Linux node-local load balancing setup.

1.  Create the configuration directory:
    ```powershell
    New-Item -ItemType Directory -Force -Path "C:\etc\envoy"
    New-Item -ItemType Directory -Force -Path "C:\var\log\envoy"
    ```

2.  Create `C:\etc\envoy\envoy.yaml`. This is the bootstrap config.
    (See [examples/windows-envoy/envoy.yaml](examples/windows-envoy/envoy.yaml))

3.  Create `C:\etc\envoy\cds.yaml`. This file defines the upstream control plane servers.
    **Important:** You must edit this file to include the IP addresses of your k0s controllers.
    (See [examples/windows-envoy/cds.yaml](examples/windows-envoy/cds.yaml))

    Example snippet for `cds.yaml` upstream endpoints:
    ```yaml
    load_assignment:
      cluster_name: apiserver
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: "192.168.1.10" # Controller 1 IP
                port_value: 6443
        - endpoint:
            address:
              socket_address:
                address: "192.168.1.11" # Controller 2 IP
                port_value: 6443
    ```

## Step 3: Install Envoy as a Service

Use the provided PowerShell script or `nssm` manually to install Envoy as a background service.

Using the example script:
```powershell
./examples/windows-envoy/install-envoy.ps1
```

Or manually:
```powershell
nssm install envoy "C:\Program Files\envoy\envoy.exe" "-c C:\etc\envoy\envoy.yaml --service-cluster nllb-cluster --service-node nllb-node"
nssm set envoy AppStdout "C:\var\log\envoy\envoy.log"
nssm set envoy AppStderr "C:\var\log\envoy\envoy.err.log"
nssm set envoy AppRotateFiles 1
nssm set envoy Start SERVICE_AUTO_START
nssm start envoy
```

Verify that Envoy is running and listening on port 7443:
```powershell
netstat -an | findstr 7443
```

## Step 4: Join the Windows Worker

To force the Windows worker to use the local Envoy proxy, we need to modify the bootstrap configuration.

1.  **Generate a Join Token:**
    On a controller node:
    ```bash
    k0s token create --role=worker --expiry=100h > worker.token
    ```

2.  **Modify the Token (Kubeconfig):**
    The token is a base64-encoded Kubeconfig.
    *   Decode the token.
    *   Change the `server` URL from the controller's IP to `https://localhost:7443`.
    *   Re-encode it (optional, or just save as `bootstrap.kubeconfig`).

    *Alternatively*, if you are comfortable with `k0s install`, you can let k0s generate the config and then modify it before starting the service, but providing a modified kubeconfig is cleaner.

3.  **Install/Run k0s Worker:**

    If using the modified kubeconfig file:
    ```powershell
    k0s.exe install worker --token-file bootstrap.kubeconfig
    k0s.exe start
    ```

    Now, the k0s worker component (Kubelet/Konnectivity) will connect to `https://localhost:7443`, which Envoy forwards to one of the upstream controllers defined in `cds.yaml`.

## limitations

*   **Manual Management:** Currently, the `cds.yaml` file is static. If controllers change IPs, you must update this file on Windows nodes manually or via automation. (Linux nodes use a dynamic reconciler).
*   **Certificates:** Envoy simply passes through TCP traffic (SNI), so the API server certificates must be valid for the name/IP the worker uses, or the worker must be configured to trust them. Since we are using `localhost`, ensure your API server cert SANS don't strictly block this, or that `insecure-skip-tls-verify` is handled if necessary (though k0s usually handles node bootstrapping securely).
