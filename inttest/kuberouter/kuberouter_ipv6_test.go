// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kuberouter

import (
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type KubeRouterIPv6Suite struct {
	common.BootlooseSuite
	mode string // "dual-stack" or "single-stack"
}

func (s *KubeRouterIPv6Suite) TestIPv6Networking() {
	clusterCfg := &v1beta1.ClusterConfig{
		Spec: &v1beta1.ClusterSpec{
			Network: func() *v1beta1.Network {
				network := v1beta1.DefaultNetwork()
				network.Provider = "kuberouter"
				if s.mode == "dual-stack" {
					network.DualStack.Enabled = true
					network.DualStack.IPv6PodCIDR = "fd00::/108"
					network.DualStack.IPv6ServiceCIDR = "fd01::/108"
					network.PodCIDR = "10.244.0.0/16"
					network.ServiceCIDR = "10.96.0.0/12"
				} else { // single-stack
					network.PodCIDR = "fd00::/108"
					network.ServiceCIDR = "fd01::/108"
				}
				return network
			}(),
		},
	}

	config, err := yaml.Marshal(clusterCfg)
	s.Require().NoError(err)
	s.WriteFileContent(s.ControllerNode(0), "/tmp/k0s.yaml", config)

	// Enable IPv6SingleStack feature gate for single-stack mode
	args := []string{"--config=/tmp/k0s.yaml", "--disable-components=konnectivity-server,metrics-server"}
	if s.mode == "single-stack" {
		args = append(args, "--feature-gates=IPv6SingleStack=true")
	}
	s.Require().NoError(s.InitController(0, args...))

	if s.mode == "single-stack" {
		s.T().Log("Setting up IPv6 DNS for workers")
		common.ConfigureIPv6ResolvConf(&s.BootlooseSuite)
	}

	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient("controller0", "")
	s.Require().NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.Require().NoError(err)

	err = s.WaitForNodeReady("worker1", kc)
	s.Require().NoError(err)

	s.T().Log("waiting to see kube-router pods ready")
	s.Require().NoError(common.WaitForKubeRouterReady(s.Context(), kc), "kube-router did not start")

	// Create test pods
	testPod1 := `
apiVersion: v1
kind: Pod
metadata:
  name: ipv6-test-pod-1
  namespace: default
  labels:
    app: ipv6-test
spec:
  containers:
  - name: nginx
    image: docker.io/library/nginx:1.29.3-alpine
    ports:
    - containerPort: 80
  nodeSelector:
    kubernetes.io/hostname: worker0
`

	testPod2 := `
apiVersion: v1
kind: Pod
metadata:
  name: ipv6-test-pod-2
  namespace: default
  labels:
    app: ipv6-test
spec:
  containers:
  - name: curl
    image: docker.io/curlimages/curl:8.17.0
    command: ["/bin/sh", "-c"]
    args: ["tail -f /dev/null"]
  nodeSelector:
    kubernetes.io/hostname: worker1
`

	s.PutFile(s.ControllerNode(0), "/var/lib/k0s/manifests/test/pod1.yaml", testPod1)
	s.PutFile(s.ControllerNode(0), "/var/lib/k0s/manifests/test/pod2.yaml", testPod2)

	s.T().Log("waiting for test pods to be ready")
	s.Require().NoError(common.WaitForPod(s.Context(), kc, "ipv6-test-pod-1", metav1.NamespaceDefault))
	s.Require().NoError(common.WaitForPod(s.Context(), kc, "ipv6-test-pod-2", metav1.NamespaceDefault))

	// Test IPv6 connectivity
	s.Run("test_ipv6_connectivity", func() {
		pod1, err := kc.CoreV1().Pods(metav1.NamespaceDefault).Get(s.Context(), "ipv6-test-pod-1", metav1.GetOptions{})
		s.Require().NoError(err)

		if s.mode == "dual-stack" {
			s.Require().Len(pod1.Status.PodIPs, 2, "Pod should have both IPv4 and IPv6 addresses in dual-stack mode")
		} else {
			s.Require().Len(pod1.Status.PodIPs, 1, "Pod should have only IPv6 address in single-stack mode")
		}

		// Test connectivity using IPv6
		ipv6Addr := ""
		for _, ip := range pod1.Status.PodIPs {
			if ip.IP[0:2] == "fd" { // IPv6 address starting with fd
				ipv6Addr = ip.IP
				break
			}
		}
		s.Require().NotEmpty(ipv6Addr, "Should find IPv6 address for pod")

		s.T().Logf("Testing connectivity to IPv6 address: %s", ipv6Addr)

		// Test connectivity from pod2 to pod1 using IPv6
		err = wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
			output, err := common.PodExecCmdOutput(kc, nil, "ipv6-test-pod-2", metav1.NamespaceDefault, fmt.Sprintf("curl -s --connect-timeout 5 http://[%s]", ipv6Addr))
			if err != nil {
				s.T().Logf("Connectivity test failed: %v, output: %s", err, output)
				return false, nil
			}
			return true, nil
		})
		s.Require().NoError(err, "IPv6 connectivity test failed")
	})
}

func TestKubeRouterIPv6Suite(t *testing.T) {
	testCases := []struct {
		name string
		mode string
	}{
		{"dual-stack", "dual-stack"},
		{"single-stack", "single-stack"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := KubeRouterIPv6Suite{
				BootlooseSuite: common.BootlooseSuite{
					ControllerCount: 1,
					WorkerCount:     2,
				},
				mode: tc.mode,
			}

			if tc.mode == "single-stack" {
				s.Networks = []string{"bridge-ipv6"}
				s.AirgapImageBundleMountPoints = []string{"/var/lib/k0s/images/bundle.tar"}
				s.K0sExtraImageBundleMountPoints = []string{"/var/lib/k0s/images/ipv6.tar"}
			}

			suite.Run(t, &s)
		})
	}
}
