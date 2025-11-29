<#
.SYNOPSIS
    Installs Envoy as a Windows Service for k0s Node-Local Load Balancing.
.DESCRIPTION
    This script configures Envoy to run as a Windows service using NSSM.
    It assumes you have:
    1. 'nssm' installed and in your PATH.
    2. 'envoy.exe' available (or instructions to fetch it are provided).
    3. 'envoy.yaml' and 'cds.yaml' in the current directory.
#>

param (
    [string]$EnvoyExePath = "C:\Program Files\envoy\envoy.exe",
    [string]$ConfigDir = "C:\etc\envoy",
    [string]$LogDir = "C:\var\log\envoy"
)

$ErrorActionPreference = "Stop"

# 1. check prerequisites
if (-not (Get-Command nssm -ErrorAction SilentlyContinue)) {
    Write-Error "NSSM is not found in PATH. Please install it (e.g. 'choco install nssm')."
}

if (-not (Test-Path $EnvoyExePath)) {
    Write-Warning "Envoy binary not found at $EnvoyExePath."
    Write-Warning "To obtain envoy.exe, you can extract it from the official Docker image using 'crane':"
    Write-Warning "  crane export --platform windows/amd64 docker.io/envoyproxy/envoy-windows-ltsc2022:v1.25.2 envoy.tar"
    Write-Warning "  tar xf envoy.tar"
    Write-Warning "  Move 'Files/Program Files/envoy/envoy.exe' to '$EnvoyExePath'"
    
    # Create the directory anyway so the user can drop the file there
    New-Item -ItemType Directory -Force -Path (Split-Path $EnvoyExePath -Parent) | Out-Null
}

# 2. Create directories
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

# 3. Copy configuration files
Write-Host "Copying configuration files to $ConfigDir..."
Copy-Item -Path ".\envoy.yaml" -Destination "$ConfigDir\envoy.yaml" -Force
Copy-Item -Path ".\cds.yaml" -Destination "$ConfigDir\cds.yaml" -Force

# 4. Install Service via NSSM
Write-Host "Installing 'envoy' service..."
# Remove existing service if present (optional, careful!)
# nssm remove envoy confirm

nssm install envoy $EnvoyExePath "-c $ConfigDir\envoy.yaml --service-cluster nllb-cluster --service-node nllb-node --log-path $LogDir\envoy.log"
if ($LASTEXITCODE -ne 0) {
    Write-Warning "Service installation might have failed or service already exists."
}

# 5. Configure Service Logging & Rotation
Write-Host "Configuring service parameters..."
nssm set envoy AppStdout "$LogDir\envoy.log"
nssm set envoy AppStderr "$LogDir\envoy.err.log"
nssm set envoy AppStdoutCreationDisposition 4
nssm set envoy AppStderrCreationDisposition 4
nssm set envoy AppRotateFiles 1
nssm set envoy AppRotateOnline 1
nssm set envoy AppRotateSeconds 0
nssm set envoy AppRotateBytes 1000000 # 1MB

# 6. Configure Restart Policy
nssm set envoy AppExit Default Restart
nssm set envoy AppRestartDelay 3
nssm set envoy Start SERVICE_AUTO_START

Write-Host "Envoy service installed and configured."
Write-Host "You can start it with: nssm start envoy"
