# Falco Quick Start Guide

## Install Everything (One Command)

```bash
sudo bash security/setup-falco.sh
```

**That's it!** This installs Falco, configures it, and starts monitoring.

## What You Get

✅ **18 Security Rules** monitoring:
- SPIRE Workload API security
- mTLS server behavior
- Container security
- Network anomalies
- Certificate operations
- Kubernetes resource access

✅ **Real-time Alerts** via:
- systemd journal (`journalctl`)
- JSON log file (`/var/log/falco.log`)

✅ **Auto-configured** for your system:
- Tests modern_ebpf, ebpf, and kmod drivers
- Uses whichever works best
- Enables JSON output
- Sets up file logging

## View Alerts

```bash
# Live alerts
sudo journalctl -u falco-modern-bpf.service -f

# JSON logs
tail -f /var/log/falco.log | jq .

# Recent alerts
sudo journalctl -u falco-modern-bpf.service -n 50
```

## Test It Works

```bash
bash security/test-falco.sh
```

## Deploy SPIRE

```bash
make minikube-up
kubectl apply -f examples/mtls-server.yaml
```

## Files

| File | Purpose |
|------|---------|
| `setup-falco.sh` | Main installation (run this) |
| `test-falco.sh` | Test rules |
| `falco_rules.yaml` | 18 custom rules |
| `FALCO_GUIDE.md` | Full documentation |
| `README.md` | Overview |

## Uninstall

```bash
sudo systemctl stop falco-modern-bpf.service
sudo systemctl disable falco-modern-bpf.service
sudo apt remove falco
```

---

**Need help?** See `FALCO_GUIDE.md` for detailed documentation.
