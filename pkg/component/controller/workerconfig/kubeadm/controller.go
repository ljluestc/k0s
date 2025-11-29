// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/token"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/sirupsen/logrus"
)

// Controller manages kubeadm bootstrap tokens and updates worker configurations
type Controller struct {
	log            logrus.FieldLogger
	clientFactory  kubernetes.ClientFactoryInterface
	tokenManager   *token.Manager
	lastToken      string
	lastTokenTime  time.Time
}

// NewController creates a new kubeadm bootstrap controller
func NewController(clientFactory kubernetes.ClientFactoryInterface) (*Controller, error) {
	client, err := clientFactory.GetClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes client: %w", err)
	}

	tokenManager, err := token.NewManagerForClient(client)
	if err != nil {
		return nil, fmt.Errorf("failed to create token manager: %w", err)
	}

	return &Controller{
		log:           logrus.WithFields(logrus.Fields{"component": "kubeadm-bootstrap-controller"}),
		clientFactory: clientFactory,
		tokenManager:  tokenManager,
	}, nil
}

var _ manager.Component = (*Controller)(nil)

// Init implements manager.Component
func (c *Controller) Init(context.Context) error {
	c.log.Info("Initializing kubeadm bootstrap controller")
	return nil
}

// Start implements manager.Component
func (c *Controller) Start(ctx context.Context) error {
	c.log.Info("Starting kubeadm bootstrap controller")

	// Start the token refresh loop
	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		if err := c.ensureValidToken(ctx); err != nil {
			c.log.WithError(err).Error("Failed to ensure valid bootstrap token")
		}
	}, 1*time.Hour) // Check token validity every hour

	return nil
}

// Stop implements manager.Component
func (c *Controller) Stop() error {
	c.log.Info("Stopping kubeadm bootstrap controller")
	return nil
}

// ensureValidToken ensures there's a valid bootstrap token for kubeadm workers
func (c *Controller) ensureValidToken(ctx context.Context) error {
	// Check if we need a new token (current token is older than 23 hours)
	if c.lastToken != "" && time.Since(c.lastTokenTime) < 23*time.Hour {
		return nil // Token is still valid
	}

	c.log.Info("Creating new bootstrap token for kubeadm workers")

	// Create a new token with 24 hour validity
	newToken, err := c.tokenManager.Create(ctx, 24*time.Hour, token.RoleWorker)
	if err != nil {
		return fmt.Errorf("failed to create bootstrap token: %w", err)
	}

	c.lastToken = newToken.String()
	c.lastTokenTime = time.Now()

	c.log.WithField("token", newToken.ID).Info("Created new bootstrap token")

	// Update the kubeadm worker config ConfigMap with the new token
	return c.updateWorkerConfig(ctx, newToken.String())
}

// updateWorkerConfig updates the kubeadm worker configuration ConfigMap with bootstrap info
func (c *Controller) updateWorkerConfig(ctx context.Context, tokenStr string) error {
	client, err := c.clientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get client: %w", err)
	}

	// Get CA cert hash for discovery
	caCertHash, err := getCACertHash(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to get CA cert hash: %w", err)
	}

	// Get API server endpoints
	apiServers, err := getAPIServerEndpoints(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to get API server endpoints: %w", err)
	}

	if len(apiServers) == 0 {
		return fmt.Errorf("no API server endpoints found")
	}

	bootstrapConfig := &BootstrapConfig{
		APIServerEndpoint: apiServers[0],
		Token:             tokenStr,
		CACertHash:        caCertHash,
	}

	configMapName := "k0s-worker-config-kubeadm-v1.32.0" // Adjust version as needed

	// Get existing ConfigMap
	configMap, err := client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap %s: %w", configMapName, err)
	}

	// Update the kubeadmBootstrap data
	bootstrapYAML, err := bootstrapConfig.ToYAML()
	if err != nil {
		return fmt.Errorf("failed to marshal bootstrap config: %w", err)
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}
	configMap.Data["kubeadmBootstrap"] = string(bootstrapYAML)

	// Update the ConfigMap
	_, err = client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap %s: %w", configMapName, err)
	}

	c.log.Info("Updated kubeadm worker configuration with new bootstrap token")
	return nil
}

// getAPIServerEndpoints retrieves the API server endpoints from the kubernetes service
func getAPIServerEndpoints(ctx context.Context, client kubernetes.Interface) ([]string, error) {
	endpoints, err := client.CoreV1().Endpoints(metav1.NamespaceDefault).Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes endpoints: %w", err)
	}

	var apiServers []string
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			host := address.IP
			if address.Hostname != "" {
				host = address.Hostname
			}
			for _, port := range subset.Ports {
				if port.Name == "https" && port.Protocol == corev1.ProtocolTCP {
					apiServers = append(apiServers, fmt.Sprintf("%s:%d", host, port.Port))
				}
			}
		}
	}

	return apiServers, nil
}
