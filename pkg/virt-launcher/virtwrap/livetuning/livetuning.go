/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Authors.
 *
 */

package livetuning

import (
	"encoding/json"
	"fmt"

	v1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/api"
)

const Annotation = "kubevirt.jimyag.com/live-tuning.v1"

type Config struct {
	CPU     *CPUTuning     `json:"cpu,omitempty"`
	Network *NetworkTuning `json:"network,omitempty"`
}

type CPUTuning struct {
	VCPUPeriod *uint64 `json:"vcpuPeriod,omitempty"`
	VCPUQuota  *int64  `json:"vcpuQuota,omitempty"`
}

type NetworkTuning struct {
	Interfaces map[string]InterfaceTuning `json:"interfaces,omitempty"`
}

type InterfaceTuning struct {
	Inbound  *BandwidthParams `json:"inbound,omitempty"`
	Outbound *BandwidthParams `json:"outbound,omitempty"`
}

type BandwidthParams struct {
	Average *uint `json:"average,omitempty"`
	Peak    *uint `json:"peak,omitempty"`
	Burst   *uint `json:"burst,omitempty"`
}

func FromVMI(vmi *v1.VirtualMachineInstance) (*Config, error) {
	if vmi == nil || vmi.Annotations == nil {
		return nil, nil
	}

	raw, exists := vmi.Annotations[Annotation]
	if !exists || raw == "" {
		return nil, nil
	}

	var config Config
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return nil, fmt.Errorf("failed to parse %s annotation: %w", Annotation, err)
	}

	if err := validate(config); err != nil {
		return nil, fmt.Errorf("invalid %s annotation: %w", Annotation, err)
	}

	return &config, nil
}

func ApplyToDomain(domain *api.Domain, config *Config) {
	if domain == nil || config == nil || config.Network == nil {
		return
	}

	for i := range domain.Spec.Devices.Interfaces {
		iface := &domain.Spec.Devices.Interfaces[i]
		if iface.Alias == nil {
			continue
		}

		ifaceTuning, exists := config.Network.Interfaces[iface.Alias.GetName()]
		if !exists {
			continue
		}

		iface.BandWidth = toLibvirtBandwidth(ifaceTuning)
	}
}

func toLibvirtBandwidth(tuning InterfaceTuning) *api.BandWidth {
	if tuning.Inbound == nil && tuning.Outbound == nil {
		return nil
	}

	return &api.BandWidth{
		Inbound:  toLibvirtBandwidthParams(tuning.Inbound),
		Outbound: toLibvirtBandwidthParams(tuning.Outbound),
	}
}

func toLibvirtBandwidthParams(params *BandwidthParams) *api.BandwidthParams {
	if params == nil {
		return nil
	}

	return &api.BandwidthParams{
		Average: *params.Average,
		Peak:    *params.Peak,
		Burst:   *params.Burst,
	}
}

func validate(config Config) error {
	if config.CPU != nil {
		if err := validateCPU(*config.CPU); err != nil {
			return err
		}
	}

	if config.Network != nil {
		if err := validateNetwork(*config.Network); err != nil {
			return err
		}
	}

	return nil
}

func validateCPU(cpu CPUTuning) error {
	if cpu.VCPUPeriod == nil && cpu.VCPUQuota == nil {
		return fmt.Errorf("cpu must define at least one of vcpuPeriod or vcpuQuota")
	}
	if cpu.VCPUPeriod != nil && *cpu.VCPUPeriod == 0 {
		return fmt.Errorf("cpu.vcpuPeriod must be greater than 0")
	}
	if cpu.VCPUQuota != nil && *cpu.VCPUQuota == 0 {
		return fmt.Errorf("cpu.vcpuQuota must be positive or -1")
	}
	if cpu.VCPUQuota != nil && *cpu.VCPUQuota < -1 {
		return fmt.Errorf("cpu.vcpuQuota must be positive or -1")
	}
	return nil
}

func validateNetwork(network NetworkTuning) error {
	for name, iface := range network.Interfaces {
		if name == "" {
			return fmt.Errorf("network.interfaces must not contain an empty interface name")
		}
		if iface.Inbound == nil && iface.Outbound == nil {
			return fmt.Errorf("network.interfaces.%s must define at least one of inbound or outbound", name)
		}
		if err := validateBandwidthParams(iface.Inbound, name, "inbound"); err != nil {
			return err
		}
		if err := validateBandwidthParams(iface.Outbound, name, "outbound"); err != nil {
			return err
		}
	}
	return nil
}

func validateBandwidthParams(params *BandwidthParams, ifaceName, direction string) error {
	if params == nil {
		return nil
	}

	switch {
	case params.Average == nil:
		return fmt.Errorf("network.interfaces.%s.%s.average must be set", ifaceName, direction)
	case *params.Average == 0:
		return fmt.Errorf("network.interfaces.%s.%s.average must be greater than 0", ifaceName, direction)
	case params.Peak == nil:
		return fmt.Errorf("network.interfaces.%s.%s.peak must be set", ifaceName, direction)
	case *params.Peak == 0:
		return fmt.Errorf("network.interfaces.%s.%s.peak must be greater than 0", ifaceName, direction)
	case params.Burst == nil:
		return fmt.Errorf("network.interfaces.%s.%s.burst must be set", ifaceName, direction)
	case *params.Burst == 0:
		return fmt.Errorf("network.interfaces.%s.%s.burst must be greater than 0", ifaceName, direction)
	}

	return nil
}
