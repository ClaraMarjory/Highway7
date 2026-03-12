# Highway7

IPLC landing IP management panel for AI infrastructure.

## What it does

- Manage landing servers via SSH
- One-click iptables DNAT forwarding
- Deploy Shadowsocks-Rust (method=none) on landing nodes
- Single binary, zero dependencies

## Quick Start

```bash
# Install
bash <(curl -sL https://raw.githubusercontent.com/ClaraMarjory/Highway7/main/scripts/install.sh)

# Or build from source
make build
./highway -port 8888 -pass yourpassword
```

## Tech Stack

- Go + Gin + SQLite
- iptables only (no gost/ehco/realm)
- SS with method=none (IPLC already encrypted)
- SSH remote execution

## Architecture

```
Master (this panel, on IPLC VPS)
├── Line A (IPLC) => port => DNAT => Landing X (US IP)
├── Line B (IPLC) => port => DNAT => Landing Y (TW IP)
└── Web panel :8888
```

## License

MIT
