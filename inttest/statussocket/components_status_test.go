// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0
package statussocket

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

// componentsStatusSuite boots a small HA control plane and verifies that
// k0s status exposes control-plane component activity and leader identity.
type componentsStatusSuite struct {
	common.BootlooseSuite
}

func (s *componentsStatusSuite) SetupTest() {
	s.Require().NoError(s.WaitForSSH(s.ControllerNode(0), 2*time.Minute, 1*time.Second))
}

func (s *componentsStatusSuite) TestControlPlaneStatusFields() {
	// Bootstrap first controller
	s.Require().NoError(s.InitController(0, "--disable-components=metrics-server"))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	// Create join token for additional controllers
	joinToken, err := s.GetJoinToken("controller")
	s.Require().NoError(err)

	// Join the rest of controllers
	for i := 1; i < s.ControllerCount; i++ {
		s.Require().NoError(s.InitController(i,
			"--disable-components=metrics-server",
			joinToken,
		))
		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(i)))
	}

	// Wait until API responds and nodes are visible
	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitForNodeReady(s.ControllerNode(0), kc))

	// Poll k0s status on controller0 until controlPlane fields indicate activity
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	type controlPlane struct {
		SchedulerActive           bool   `json:"schedulerActive"`
		SchedulerLeader           string `json:"schedulerLeader"`
		ControllerManagerActive   bool   `json:"controllerManagerActive"`
		ControllerManagerLeader   string `json:"controllerManagerLeader"`
	}
	type statusPayload struct {
		ControlPlane controlPlane `json:"controlPlane"`
	}

	s.Require().Eventually(func() bool {
		output, err := ssh.ExecWithOutput(s.Context(), "k0s status -o json")
		if err != nil {
			return false
		}
		var payload statusPayload
		if err := json.Unmarshal([]byte(output), &payload); err != nil {
			return false
		}
		cp := payload.ControlPlane
		// We only need to ensure there is an active leader for each component somewhere.
		// The leader identity is informative but should not be empty once active is true.
		return cp.SchedulerActive && cp.ControllerManagerActive && cp.SchedulerLeader != "" && cp.ControllerManagerLeader != ""
	}, 2*time.Minute, 2*time.Second, "control-plane status did not become active")
}

func TestComponentsStatusSuite(t *testing.T) {
	suite.Run(t, &componentsStatusSuite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 3,
		},
	})
}


