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

package livetuning_test

import (
	"testing"

	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/api"
	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/livetuning"
)

func TestLiveTuningAnnotation(t *testing.T) {
	g := NewWithT(t)

	vmi := &v1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				livetuning.Annotation: `{
					"cpu":{"vcpuPeriod":100000,"vcpuQuota":200000},
					"network":{
						"interfaces":{
							"default":{
								"inbound":{"average":1024,"peak":2048,"burst":512},
								"outbound":{"average":4096,"peak":8192,"burst":1024}
							}
						}
					}
				}`,
			},
		},
	}
	domain := &api.Domain{
		Spec: api.DomainSpec{
			Devices: api.Devices{
				Interfaces: []api.Interface{
					{Alias: api.NewUserDefinedAlias("default")},
					{Alias: api.NewUserDefinedAlias("untuned")},
				},
			},
		},
	}

	config, err := livetuning.FromVMI(vmi)
	g.Expect(err).NotTo(HaveOccurred())
	livetuning.ApplyToDomain(domain, config)

	g.Expect(*config.CPU.VCPUPeriod).To(Equal(uint64(100000)))
	g.Expect(*config.CPU.VCPUQuota).To(Equal(int64(200000)))
	g.Expect(domain.Spec.Devices.Interfaces[0].BandWidth).To(Equal(&api.BandWidth{
		Inbound:  &api.BandwidthParams{Average: 1024, Peak: 2048, Burst: 512},
		Outbound: &api.BandwidthParams{Average: 4096, Peak: 8192, Burst: 1024},
	}))
	g.Expect(domain.Spec.Devices.Interfaces[1].BandWidth).To(BeNil())
}

func TestLiveTuningAnnotationRejectsInvalidCPUQuota(t *testing.T) {
	g := NewWithT(t)

	vmi := &v1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				livetuning.Annotation: `{"cpu":{"vcpuQuota":0}}`,
			},
		},
	}

	_, err := livetuning.FromVMI(vmi)
	g.Expect(err).To(MatchError(ContainSubstring("cpu.vcpuQuota must be positive or -1")))
}
