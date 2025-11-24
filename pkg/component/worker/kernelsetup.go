//go:build !linux

// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

// KernelSetup comment
func KernelSetup(config *v1beta1.KernelSpec) error { return nil }
