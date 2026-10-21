# xboard-node

Node backend for [Xboard](https://github.com/cedar2025/Xboard). Supports `sing-box` / `xray-core` dual kernels.

This is a fork of [cedar2025/Xboard-Node](https://github.com/cedar2025/Xboard-Node) with user-sync fixes (see [Changes in this fork](#changes-in-this-fork)). The installer, `xbctl upgrade` and the Docker image all download from this fork.

> **Disclaimer**: This project is for educational and learning purposes only.

## Features

- Protocols: V2Ray family, Trojan, Shadowsocks, Hysteria2, TUIC, AnyTLS
- Sync: WebSocket push + REST polling dual channel
- User controls: speed limit, device limit, alive-IP tracking, hot update
- Deploy modes: node mode, machine mode, standalone mode
- Multi-instance: single process binding multiple panels / nodes

## Install

### Installer (Linux systemd)

```bash
# Node mode
curl -fsSL https://raw.githubusercontent.com/konosola-dcl/xb-node/dev/install.sh | \
  sudo bash -s -- --mode node --panel https://panel.example.com --token TOKEN --node-id 1

# Machine mode
curl -fsSL https://raw.githubusercontent.com/konosola-dcl/xb-node/dev/install.sh | \
  sudo bash -s -- --mode machine --panel https://panel.example.com --token TOKEN --machine-id 1
```

Add `--kernel xray` to use xray-core (default: sing-box). Add `--version v1.13.1` to pin a release instead of the latest one.

### Upgrade

Existing nodes switch to this fork with one command, including nodes installed from upstream cedar2025:

```bash
curl -fsSL https://raw.githubusercontent.com/konosola-dcl/xb-node/dev/install.sh | sudo bash -s -- upgrade
```

The upgrade does the following:

- Replaces `xboard-node` and `xbctl`.
- Keeps `/etc/xboard-node/config.yml`.
- Backs up the old files to `/etc/xboard-node/backups/`.
- Restarts the service.

Check the running version with `xboard-node -v`.

Nodes installed from upstream need this installer once. Their `xbctl upgrade` still downloads from upstream. After the switch, `xbctl upgrade` downloads from this fork.

### Docker

```bash
docker run -d --restart=always --network=host \
  -e apiHost=https://panel.com -e apiKey=TOKEN -e nodeID=1 \
  ghcr.io/konosola-dcl/xboard-node:latest
```

### Docker Compose

The `compose` branch only exists upstream. Clone it and point the image at this fork:

```bash
git clone -b compose --depth 1 https://github.com/cedar2025/xboard-node.git
cd xboard-node
sed -i 's#ghcr.io/cedar2025/#ghcr.io/konosola-dcl/#' compose.yml
vim config/config.yml   # set panel.url / token / node_id
docker compose up -d
```

## xbctl

Run `xbctl` after installation for help. Common commands:

```bash
xbctl list                          # list all instances
xbctl status                        # running status
xbctl bind add-node --panel URL --token TOKEN --node-id 1
xbctl bind add-machine --panel URL --token TOKEN --machine-id 1
xbctl bind remove-node --panel URL --node-id 1
xbctl service restart
```

## Configuration

Legacy single-panel config is fully compatible. Appending bindings auto-migrates to `instances` format. See `config.yml.example`.

## Extensions

- Custom routes: [docs-custom-routes.md](docs-custom-routes.md)
- Custom outbounds: [docs-custom-outbounds.md](docs-custom-outbounds.md)
- DNS providers (ACME DNS-01): [docs-dns-providers.md](docs-dns-providers.md)

## Changes in this fork

- **xray applies new UUIDs on full syncs.** Full syncs happen during REST polling and in machine mode. When a user reset their subscription, xray logged `User user@ID already exists` and kept accepting the old UUID until the node restarted. It now removes the old entry first.
- **The kernel starts when users arrive.** The kernel stops when a node has no users, for example a new node or after its last user expired. It used to drop the next users until a restart. It now starts with them.
- **sing-box `AddUsers` replaces a user whose UUID changed** instead of skipping them as a duplicate.

## Releasing

The binaries come from this fork's GitHub Releases. `.github/workflows/ci.yml` builds them.

1. Enable Actions on the fork once: open the Actions tab and enable workflows. GitHub disables workflows on forks by default.
2. Push a tag:

   ```bash
   git tag v1.13.1
   git push origin v1.13.1
   ```

   CI runs the tests, builds `linux-amd64` and `linux-arm64`, and publishes the release as latest. The installer and `xbctl upgrade` download the latest release by default.
3. Every push to `dev` also updates a rolling `dev` release. Install it with `--version dev`.

Docker images go to `ghcr.io/konosola-dcl/xboard-node`. If `docker pull` asks for credentials, make the package public in its GitHub package settings.

## License

MPL-2.0.
