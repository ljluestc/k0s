//go:build unix

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPopulateControlPlaneStatus_SingleNode(t *testing.T) {
	// Test single-node scenario where leader election is disabled
	status := &K0sStatus{
		Role:       "controller",
		SingleNode: true,
		K0sVars: &config.CfgVars{
			AdminKubeConfigPath: "/tmp/nonexistent",
		},
	}

	handler := &statusHandler{
		Status: &Status{},
	}

	err := handler.populateControlPlaneStatus(context.Background(), status)
	require.NoError(t, err)

	// Should be active and leader should be hostname
	assert.True(t, status.ControlPlane.SchedulerActive)
	assert.True(t, status.ControlPlane.ControllerManagerActive)

	hostname, _ := os.Hostname()
	assert.Equal(t, hostname, status.ControlPlane.SchedulerLeader)
	assert.Equal(t, hostname, status.ControlPlane.ControllerManagerLeader)
}

func TestPopulateControlPlaneStatus_NonJoinableStorage(t *testing.T) {
	// Test non-joinable storage scenario where leader election is disabled
	status := &K0sStatus{
		Role:       "controller",
		SingleNode: false,
		ClusterConfig: &v1beta1.ClusterConfig{
			Spec: &v1beta1.ClusterSpec{
				Storage: &v1beta1.StorageSpec{
					Type: "kine",
					Kine: &v1beta1.KineConfig{
						DataSource: "sqlite:///tmp/test.db",
					},
				},
			},
		},
		K0sVars: &config.CfgVars{
			AdminKubeConfigPath: "/tmp/nonexistent",
		},
	}

	handler := &statusHandler{
		Status: &Status{},
	}

	err := handler.populateControlPlaneStatus(context.Background(), status)
	require.NoError(t, err)

	// Should be active and leader should be hostname
	assert.True(t, status.ControlPlane.SchedulerActive)
	assert.True(t, status.ControlPlane.ControllerManagerActive)

	hostname, _ := os.Hostname()
	assert.Equal(t, hostname, status.ControlPlane.SchedulerLeader)
	assert.Equal(t, hostname, status.ControlPlane.ControllerManagerLeader)
}


func TestControlPlaneStatus_JSON_Serialization(t *testing.T) {
	status := ControlPlaneStatus{
		SchedulerActive:         true,
		SchedulerLeader:         "controller-0",
		ControllerManagerActive: false,
		ControllerManagerLeader: "",
	}

	// Test JSON marshaling
	data, err := json.Marshal(status)
	require.NoError(t, err)

	expected := `{"schedulerActive":true,"schedulerLeader":"controller-0","controllerManagerActive":false}`
	assert.JSONEq(t, expected, string(data))

	// Test JSON unmarshaling
	var unmarshaled ControlPlaneStatus
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, status, unmarshaled)
}
