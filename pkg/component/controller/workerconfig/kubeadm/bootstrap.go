// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/net"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/token"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	bootstraptokenv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/bootstraptoken/v1"
	"sigs.k8s.io/yaml"
)

// BootstrapConfig represents the configuration needed for kubeadm join
type BootstrapConfig struct {
	APIServerEndpoint string `json:"apiServerEndpoint"`
	Token             string `json:"token"`
	CACertHash        string `json:"caCertHash"`
}

// GenerateBootstrapConfig creates a bootstrap configuration for kubeadm workers
func GenerateBootstrapConfig(ctx context.Context, clientFactory kubernetes.ClientFactoryInterface, apiServers []net.HostPort) (*BootstrapConfig, error) {
	client, err := clientFactory.GetClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes client: %w", err)
	}

	// Create a bootstrap token for workers
	tokenManager, err := token.NewManagerForClient(client)
	if err != nil {
		return nil, fmt.Errorf("failed to create token manager: %w", err)
	}
	bootstrapToken, err := tokenManager.Create(ctx, 24*time.Hour, token.RoleWorker)
	if err != nil {
		return nil, fmt.Errorf("failed to create bootstrap token: %w", err)
	}

	// Get CA certificate hash for discovery
	caCertHash, err := getCACertHash(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get CA certificate hash: %w", err)
	}

	// Use the first API server endpoint
	var apiServerEndpoint string
	if len(apiServers) > 0 {
		apiServerEndpoint = apiServers[0].String()
	} else {
		return nil, fmt.Errorf("no API server endpoints available")
	}

	return &BootstrapConfig{
		APIServerEndpoint: apiServerEndpoint,
		Token:             bootstrapToken.String(),
		CACertHash:        caCertHash,
	}, nil
}

// getCACertHash retrieves the CA certificate hash for kubeadm discovery
func getCACertHash(ctx context.Context, client kubernetes.Interface) (string, error) {
	// Get the CA certificate from the kube-system namespace
	secret, err := client.CoreV1().Secrets(metav1.NamespaceSystem).Get(ctx, "k0s-ca", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get CA secret: %w", err)
	}

	caCert, ok := secret.Data["ca.crt"]
	if !ok {
		return "", fmt.Errorf("CA certificate not found in secret")
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256(caCert)
	return fmt.Sprintf("sha256:%x", hash), nil
}

// ToYAML converts the bootstrap config to YAML format for ConfigMap storage
func (c *BootstrapConfig) ToYAML() ([]byte, error) {
	return yaml.Marshal(c)
}
