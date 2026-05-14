# Live tuning annotation

This document describes the downstream live tuning annotation:

```yaml
kubevirt.jimyag.com/live-tuning.v1
```

The annotation is a structured JSON payload on a `VirtualMachineInstance`. It is
consumed by virt-launcher during `SyncVMI` and applies selected runtime tuning to
the libvirt domain without allowing arbitrary XML patches.

## Example

```yaml
metadata:
  annotations:
    kubevirt.jimyag.com/live-tuning.v1: |
      {
        "cpu": {
          "vcpuPeriod": 100000,
          "vcpuQuota": 200000
        },
        "network": {
          "interfaces": {
            "default": {
              "inbound": {"average": 1024, "peak": 2048, "burst": 512},
              "outbound": {"average": 1024, "peak": 2048, "burst": 512}
            }
          }
        }
      }
```

## CPU tuning

CPU tuning supports:

- `cpu.vcpuPeriod`: vCPU CFS period in microseconds. It must be greater than 0.
- `cpu.vcpuQuota`: vCPU CFS quota in microseconds. It must be positive or `-1`.

The effective CPU limit is:

```text
vcpuQuota / vcpuPeriod
```

For example, `vcpuPeriod=100000` and `vcpuQuota=200000` allows roughly 2 CPUs.
Use `vcpuQuota=-1` to remove the quota limit.

virt-launcher applies CPU tuning through libvirt scheduler parameters with live
and config impact.

## Network bandwidth tuning

Network tuning supports bandwidth limits per interface name:

```json
{
  "network": {
    "interfaces": {
      "default": {
        "inbound": {"average": 1024, "peak": 2048, "burst": 512},
        "outbound": {"average": 1024, "peak": 2048, "burst": 512}
      }
    }
  }
}
```

The interface key is the KubeVirt interface name, for example `default`.

Each configured direction must define all three values:

- `average`: rate in KiB/s
- `peak`: peak rate in KiB/s
- `burst`: burst size in KiB

virt-launcher maps these values to libvirt interface `<bandwidth>` XML and
updates the matching interface with `UpdateDeviceFlags` using live and config
impact.

## Notes

- The annotation is intentionally limited to a small whitelist of tunables.
- Invalid JSON or invalid values fail `SyncVMI`.
- CPU quota is throttling, not physical CPU frequency scaling.
- Libvirt support depends on the active driver and mode. For example, interface
  bandwidth tuning may be unavailable in `qemu:///session`.
- Manual `virsh` changes are not persisted through KubeVirt reconciliation,
  restart, or migration. Prefer this annotation for repeatable testing.
