# Proxmaid v2

A storage management sidecar for Proxmox VE, powered by [NonRAID](https://github.com/qvr/nonraid).

Proxmaid eliminates the need to run Unraid as a VM inside Proxmox by providing native, parity-protected storage management with a modern web interface.

## Project Structure

```
proxmaid-v2/
├── api/            # Go backend (REST API)
│   ├── cmd/        # Entry points
│   └── internal/   # Core packages (array, system, api)
├── ui/             # Next.js frontend (coming soon)
├── pve-plugin/     # Proxmox storage plugin (Perl)
├── nonraid/        # NonRAID kernel driver source
└── docs/           # Documentation & PRD
```

## Quick Start (Development)

```bash
# Run the API server (auto-enables mock mode without hardware)
cd api && go run ./cmd/proxmaid/

# Run tests
cd api && go test ./... -v
```

The API listens on port **8484** by default. Override with `PROXMAID_PORT` env var.

## API Endpoints

| Method | Path                | Description              |
|--------|---------------------|--------------------------|
| GET    | `/api/health`       | Health check             |
| GET    | `/api/array/status` | Array status (JSON)      |
| POST   | `/api/array/start`  | Start the array          |
| POST   | `/api/array/stop`   | Stop the array           |
| POST   | `/api/array/check`  | Start parity check       |
| GET    | `/api/system/module`| Kernel module status     |

## Documentation

- [Product Requirements Document](docs/PRD.md)

## License
TBD
