// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/sirupsen/logrus"
)

// check if kernel has overlay fs
func hasFilesystem(filesystem string) bool {
	data, err := os.ReadFile("/proc/filesystems")
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == filesystem {
			return true
		}
	}
	return false
}

func modprobe(module string) error {
	out, err := exec.Command("modprobe", module).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to load kernel module %s: %s: %w", module, out, err)
	}
	return nil
}

func setSysCtl(key, value string) error {
	pathComp := strings.ReplaceAll(key, ".", "/")
	file := path.Join("/proc", "sys", pathComp)
	if err := os.WriteFile(file, []byte(value), 0644); err != nil {
		return fmt.Errorf("failed to enable %s=%s: %w", file, value, err)
	}
	return nil
}

// KernelSetup sets the needed kernel tuning params. If setting the options fails, it only logs
// a warning but does not prevent the starting of worker
func KernelSetup(config *v1beta1.KernelSpec) error {
	if !hasFilesystem("overlay") {
		if err := modprobe("overlay"); err != nil {
			logrus.WithError(err).Warn("Failed to load overlay module")
		}
	}
	if !file.Exists("/proc/net/nf_conntrack") {
		if err := modprobe("nf_conntrack"); err != nil {
			logrus.WithError(err).Warn("Failed to load nf_conntrack module")
		}
	}
	if !file.Exists("/proc/sys/net/bridge/bridge-nf-call-iptables") {
		if err := modprobe("br_netfilter"); err != nil {
			logrus.WithError(err).Warn("Failed to load br_netfilter module")
		}
	}
	// https://github.com/kubernetes/kubernetes/issues/108877
	if !file.Exists("/proc/net/ip_tables_targets") {
		if err := modprobe("ip_tables"); err != nil {
			logrus.WithError(err).Warn("Failed to load ip_tables module")
		}
	}
	if err := setSysCtl("net.ipv4.conf.all.forwarding", "1"); err != nil {
		logrus.WithError(err).Warn("Failed to enable net.ipv4.conf.all.forwarding")
	}
	if err := setSysCtl("net.ipv4.conf.default.forwarding", "1"); err != nil {
		logrus.WithError(err).Warn("Failed to enable net.ipv4.conf.default.forwarding")
	}
	if err := setSysCtl("net.ipv6.conf.all.forwarding", "1"); err != nil {
		logrus.WithError(err).Warn("Failed to enable net.ipv6.conf.all.forwarding")
	}
	if err := setSysCtl("net.ipv6.conf.default.forwarding", "1"); err != nil {
		logrus.WithError(err).Warn("Failed to enable net.ipv6.conf.default.forwarding")
	}
	if err := setSysCtl("net.bridge.bridge-nf-call-iptables", "1"); err != nil {
		logrus.WithError(err).Warn("Failed to enable net.bridge.bridge-nf-call-iptables")
	}
	if err := setSysCtl("net.bridge.bridge-nf-call-ip6tables", "1"); err != nil {
		logrus.WithError(err).Warn("Failed to enable net.bridge.bridge-nf-call-ip6tables")
	}

	if config != nil {
		for _, module := range config.Modules {
			if err := modprobe(module); err != nil {
				return err
			}
		}
		for k, v := range config.SysctlParams {
			if err := setSysCtl(k, v); err != nil {
				return err
			}
		}
	}
	return nil
}
