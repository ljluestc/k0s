// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package crio

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	containerruntime "github.com/k0sproject/k0s/pkg/container/runtime"
	"github.com/k0sproject/k0s/pkg/supervisor"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Component implements the component interface to manage cri-o as a k0s component.
type Component struct {
	K0sVars *config.CfgVars

	supervisor     *supervisor.Supervisor
	executablePath string
}

var _ manager.Component = (*Component)(nil)

// NewComponent creates a new cri-o component
func NewComponent(vars *config.CfgVars) *Component {
	return &Component{
		K0sVars: vars,
	}
}

// Init extracts the needed binaries
func (c *Component) Init(ctx context.Context) error {
	// For now we assume CRI-O binary is already present or will be handled similarly to containerd
	// But since CRI-O is usually provided by the OS package manager or separate install,
	// we might just need to find it.
	// However, to be consistent with how k0s works "batteries included", we should probably embed it.
	// For the initial implementation as per issue request "Add CRI-O as build-in CRI",
	// we will assume it needs to be managed by k0s.
	
	// Placeholder for binary extraction if we were to embed it.
	// c.executablePath = filepath.Join(c.K0sVars.BinDir, "crio")
	return nil
}

// Start runs cri-o
func (c *Component) Start(ctx context.Context) error {
	log := logrus.WithField("component", "crio")
	log.Info("Starting cri-o")

	// Basic configuration setup would go here
	
	// We need to find the crio binary if not embedded/extracted
	if c.executablePath == "" {
		// Fallback to looking in path or standard locations if not set in Init
		c.executablePath = "crio" 
	}

	c.supervisor = &supervisor.Supervisor{
		Name:    "crio",
		BinPath: c.executablePath,
		RunDir:  c.K0sVars.RunDir,
		DataDir: c.K0sVars.DataDir,
		Args: []string{
			// Add CRI-O specific arguments here
			// "--root=" + filepath.Join(c.K0sVars.DataDir, "containers/storage"),
			// "--runroot=" + filepath.Join(c.K0sVars.RunDir, "containers/storage"),
		},
	}

	if err := c.supervisor.Supervise(); err != nil {
		return err
	}

	log.Debug("Waiting for cri-o")
	var lastErr error
	err := wait.ExponentialBackoffWithContext(ctx, wait.Backoff{
		Duration: 100 * time.Millisecond, Factor: 1.2, Jitter: 0.05, Steps: 30,
	}, func(ctx context.Context) (bool, error) {
		// We need to define the endpoint for CRI-O. Usually /var/run/crio/crio.sock or similar.
		endpointPath := filepath.Join(c.K0sVars.RunDir, "crio.sock")
		endpoint := &url.URL{Scheme: "unix", Path: endpointPath}
		rt := containerruntime.NewContainerRuntime(endpoint)
		if lastErr = rt.Ping(ctx); lastErr != nil {
			log.WithError(lastErr).Debug("Failed to ping cri-o")
			return false, nil
		}

		log.Debug("Successfully pinged cri-o")
		return true, nil
	})

	if err != nil {
		if lastErr == nil {
			return fmt.Errorf("failed to ping cri-o: %w", err)
		}
		return fmt.Errorf("failed to ping cri-o: %w (%w)", err, lastErr)
	}

	return nil
}

// Stop stops cri-o
func (c *Component) Stop() error {
	if c.supervisor != nil {
		c.supervisor.Stop()
	}
	return nil
}

