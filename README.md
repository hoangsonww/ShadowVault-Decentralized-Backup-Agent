# ShadowVault — Decentralized Encrypted Backup Agent

<p>
<!-- Core implementation -->
<img alt="Go" src="https://img.shields.io/badge/Go-language-cyan?style=for-the-badge&logo=go&logoColor=white" />
<img alt="protobuf" src="https://img.shields.io/badge/Protobuf-serialization-blue?style=for-the-badge&logo=protobuf&logoColor=white" />
<img alt="libp2p" src="https://img.shields.io/badge/libp2p-p2p-yellow?style=for-the-badge&logo=libp2p&logoColor=white" />
<img alt="AES-256-GCM" src="https://img.shields.io/badge/AES--256--GCM-encryption-green?style=for-the-badge" />
<img alt="Content Addressed Storage" src="https://img.shields.io/badge/CAS-content--addressed_teal?style=for-the-badge&logo=database&logoColor=white" />
<img alt="Ed25519" src="https://img.shields.io/badge/Ed25519-signature-purple?style=for-the-badge" />
<img alt="Bbolt" src="https://img.shields.io/badge/Bbolt-embedded%20DB-orange?style=for-the-badge&logo=bolt&logoColor=white" />
<img alt="CLI" src="https://img.shields.io/badge/CLI-tooling-darkblue?style=for-the-badge&logo=console&logoColor=white" />
<img alt="Docker" src="https://img.shields.io/badge/Docker-container-blue?style=for-the-badge&logo=docker&logoColor=white" />
<img alt="Docker Compose" src="https://img.shields.io/badge/Docker_Compose-orchestration-236adb?style=for-the-badge&logo=docker&logoColor=white" />
<img alt="C" src="https://img.shields.io/badge/C-utility-lightgrey?style=for-the-badge&logo=c&logoColor=white" />
<img alt="makefile" src="https://img.shields.io/badge/makefile-build-orange?style=for-the-badge&logo=make&logoColor=white" />
<img alt="Testing" src="https://img.shields.io/badge/Testing-unit%20tests-blueviolet?style=for-the-badge&logo=testing&logoColor=white" />
<img alt="Shell Scripting" src="https://img.shields.io/badge/Shell_Scripting-helpers-yellow?style=for-the-badge&logo=shell&logoColor=white" />
<img alt="CLI" src="https://img.shields.io/badge/CLI-commands-blue?style=for-the-badge&logo=command-line&logoColor=white" />
<img alt="MIT License" src="https://img.shields.io/badge/License-MIT-blue?style=for-the-badge&logo=opensource&logoColor=white" />
</p>

## Table of Contents

1. [Overview](#overview)  
2. [Goals & Motivation](#goals--motivation)  
3. [Key Concepts](#key-concepts)  
   - [Core Technologies](#core-technologies)  
   - [Security Primitives](#security-primitives)  
   - [P2P Sync Model](#p2p-sync-model)  
4. [Quickstart](#quickstart)  
5. [Installation & Build](#installation--build)  
6. [Configuration](#configuration)  
7. [CLI Commands & Usage Reference](#cli-commands--usage-reference)  
8. [Snapshot Lifecycle](#snapshot-lifecycle)  
9. [Deduplication & CAS Internals](#deduplication--cas-internals)  
10. [Identity & Authentication](#identity--authentication)  
11. [Peer Management](#peer-management)  
12. [PubSub Message Formats & Validation](#pubsub-message-formats--validation)  
13. [Restore Workflow](#restore-workflow)  
14. [Testing](#testing)  
15. [Docker & Orchestration](#docker--orchestration)  
16. [Utility C Tool](#utility-c-tool)  
17. [Shell Helpers & Entry Point](#shell-helpers--entry-point)  
18. [Example File Layout After Run](#example-file-layout-after-run)  
19. [Troubleshooting & Common Issues](#troubleshooting--common-issues)
20. [Protocol Buffers](#protocol-buffers)  
    - [Generating Go bindings](#generating-go-bindings)  
    - [File-by-file summary](#file-by-file-summary)  
21. [Security Considerations](#security-considerations)
22. [Extension Points / Developer Notes](#extension-points--developer-notes)  
23. [Contributing](#contributing)  
24. [Glossary](#glossary)  
25. [License](#license)

## Overview

**ShadowVault** is a privacy-preserving, decentralized backup agent written in Go. It snapshots filesystem data, chunks and deduplicates content, encrypts everything client-side, and synchronizes encrypted chunks and metadata across a peer-to-peer network using libp2p. There is no trusted central server: peers gossip what blocks they have, fetch missing pieces directly, and validate integrity and authenticity through signatures.

## Goals & Motivation

- **Privacy-first backups**: All data is encrypted locally before storage or exchange. No peer can read your data without the passphrase.  
- **Deduplicated storage**: Content-addressed chunking avoids redundant uploads across snapshots.  
- **Decentralized sync**: Data is propagated via peer-to-peer gossip and direct fetch; no single point of failure.  
- **Verifiable history**: Snapshots are signed; block provenance is trackable.  
- **Resilience**: Peers can fetch missing chunks from multiple holders; auto discovery and NAT handling improve reachability.

## Key Concepts

### Core Technologies

- **Go**: Implementation language for the main agent, CLI, P2P, and snapshot logic.  
- **libp2p**: Used for peer discovery, pubsub gossip, direct streams (block requests), NAT traversal, and optional relaying.  
- **bbolt**: Embedded key-value store for metadata (snapshots, peer list, block indices).  
- **Content Addressed Storage (CAS)**: Chunks are stored/encrypted and addressed by their SHA-256 hash.  
- **CLI**: Commands for daemon, snapshot creation, restore, and peer management.

### Security Primitives

- **AES-256-GCM**: Authenticated encryption for chunk and snapshot payload confidentiality/integrity.  
- **Argon2id or scrypt**: Password-based key derivation for master encryption key (depending on chosen implementation).  
- **Ed25519 signatures**: Snapshot and protocol messages are signed to authenticate origin and prevent tampering.  
- **Persistent identity keys**: Libp2p private key persisted encrypted for stable peer identity.

### P2P Sync Model

- **Gossip (PubSub)**: Announcements of new snapshots and available block hashes.  
- **Direct block fetch**: If a peer lacks a chunk, it opens a libp2p stream to a known holder and requests it.  
- **Anti-entropy**: Peers reconcile missing pieces by observing announcements and querying.  
- **ACLs**: Optional admin lists controlling who can introduce peers or snapshots.

## Quickstart

```sh
# Build and start daemon, create identity and default config, snapshot a directory
./entrypoint.sh config.yaml /path/to/important/data

# List known peers
./bin/peerctl -c config.yaml -p "yourpass" list

# Add a peer (multiaddr)
./bin/peerctl add /ip4/1.2.3.4/tcp/9000/p2p/<peerID> -c config.yaml -p "yourpass"

# Restore a snapshot
./bin/restore-agent restore <snapshot-id> restored/ -c config.yaml -p "yourpass"
````

## Installation & Build

### Prerequisites

* Go 1.21+
* GCC (for the auxiliary C tool)
* Docker (optional, for containerized run)
* Make

### Local Build

```sh
git clone <repo-url> shadowvault
cd shadowvault
make build
```

This produces:

* `bin/backup-agent` — main daemon/snapshot CLI
* `bin/restore-agent` — snapshot restore CLI
* `bin/peerctl` — peer management CLI

### Run Tests

```sh
make test
```

## Configuration

Primary configuration lives in `config.yaml` (created automatically by `entrypoint.sh` if absent). Example:

```yaml
repository_path: "./data"
listen_port: 9000
peer_bootstrap:
  - "/ip4/127.0.0.1/tcp/9001/p2p/QmSomePeerID"
nat_traversal:
  enable_auto_relay: true
  enable_hole_punching: true
snapshot:
  min_chunk_size: 2048
  max_chunk_size: 65536
  avg_chunk_size: 8192
acl:
  admins:
    - "base64-ed25519-pubkey..."
```

Defaults are applied when fields are missing.

## CLI Commands & Usage Reference

### `backup-agent` (daemon & snapshot)

```sh
# Start daemon
./bin/backup-agent daemon -c config.yaml -p "passphrase"

# Take snapshot of a directory
./bin/backup-agent snapshot /path/to/dir -c config.yaml -p "passphrase"
```

### `restore-agent`

```sh
# Restore snapshot by ID to target directory
./bin/restore-agent restore <snapshot-id> <target-dir> -c config.yaml -p "passphrase"
```

### `peerctl`

```sh
# List stored/known peers
./bin/peerctl list -c config.yaml -p "passphrase"

# Add a peer by multiaddr
./bin/peerctl add /ip4/1.2.3.4/tcp/9000/p2p/<peerID> -c config.yaml -p "passphrase"

# Remove a stored peer
./bin/peerctl remove <peerID> -c config.yaml -p "passphrase"
```

Flags:

* `-c, --config` path to `config.yaml`
* `-p, --pass` encryption passphrase

## Snapshot Lifecycle

1. **Chunking**: Files in the target directory are read with content-defined chunking (configurable min/avg/max) to produce variable-sized pieces.
2. **Deduplication**: Each chunk is hashed (SHA-256) and if already present locally, skipped.
3. **Encryption**: Chunks are encrypted with AES-256-GCM using a key derived from the user passphrase.
4. **Storage**: Encrypted chunks are stored in CAS (via bbolt or on-disk object layout).
5. **Snapshot metadata**: A snapshot descriptor listing chunk hashes, parent snapshot (optional), timestamps, and provenance is assembled and signed.
6. **Announcement**: Signed snapshot and block availability are gossip-published to peers via pubsub.

## Deduplication & CAS Internals

* **Chunk Identification**: SHA-256 of encrypted chunk used as content address.
* **Storage**: Chunks stored under `objects/<first-two>/<rest>` or via key-value bucket.
* **Snapshot Metadata**: Includes chunk list, parent link, signer public key, signature, and arbitrary metadata (e.g., source path).
* **Garbage Collection**: Not automatic—implement reference counting or periodic pruning in extensions.

## Identity & Authentication

* **Persistent Identity**: Libp2p private key is created once and saved at `repository_path/identity.key`.
* **Snapshot Signatures**: Snapshots are signed with Ed25519 (embedded in `versioning.Snapshot.Signature`) and verified before acceptance.
* **ACL**: Admin public keys (base64) control who may perform peer introductions or snapshot promotion.

## Peer Management

* Stored in metadata DB (`bbolt`) under peers bucket.
* Peers can be added manually with `peerctl add` or auto-discovered via DHT/rendezvous if enabled.
* Peer removal cleans stored records but does not retroactively invalidate past data (chunks remain).

## PubSub Message Formats & Validation

Core message envelope used in gossip:

```json
{
  "type": "snapshot_announce" | "block_announce" | "peer_add" | "peer_remove",
  "payload": { /* type-specific struct */ },
  "sig": "<base64 signature over type||payload>",
  "pubkey": "<base64-ed25519 public key of signer>"
}
```

* **SnapshotAnnouncement**: Carries a full signed snapshot descriptor; peers validate the embedded signature before storing.
* **BlockAnnounce**: Informs network a peer has chunk with given hash.
* **PeerAdd / PeerRemove**: Introduce or revoke peers; include signatures to prevent spoofing.

Validation steps:

1. Decode `pubkey`, `sig`.
2. Reconstruct signing context (`type` + raw `payload`).
3. Verify signature (Ed25519).
4. Accept or reject based on ACL (for sensitive types).

## Restore Workflow

1. Specify snapshot ID to restore.
2. Snapshot is loaded and its signature verified.
3. For each chunk hash in the snapshot:

  * If present locally, use it.
  * Otherwise, consult known block announcements and attempt fetching from peers via direct stream protocol.
4. Decrypt each chunk and reconstruct files.
5. Restore filesystem metadata (mode, timestamps).

Example:

```sh
./bin/restore-agent restore snapshot-abc123 restored/ -c config.yaml -p "yourpass"
```

## Testing

Unit tests are included for critical modules:

* `internal/crypto/crypto_test.go` — encryption/decryption and hashing.
* `internal/chunker/chunker_test.go` — chunk boundary correctness and edge cases.
* `internal/identity/identity_test.go` — persistent identity creation and validation.

Run:

```sh
make test
```

Or directly:

```sh
go test ./... -v
```

## Docker & Orchestration

### Build Image

```sh
docker build -t shadowvault:latest .
```

### Run ShadowVault Daemon

```sh
docker run --rm -v "$(pwd)/data":/data -v "$(pwd)/config.yaml":/app/config.yaml:ro -e PASSPHRASE=yourpass shadowvault:latest daemon -c /app/config.yaml -p "$PASSPHRASE"
```

### Compose Two Nodes

```sh
docker compose up
```

(This uses `docker-compose.yml` to spin up `node1` and `node2`, share bootstrap configuration and run daemons.)

### Snapshot & Restore Inside Container

```sh
docker exec -it backupagent_node1 /bin/sh -c "./bin/backup-agent snapshot /data/to/backup -c /app/config.yaml -p yourpass"
docker exec -it backupagent_node1 /bin/sh -c "./bin/restore-agent restore <snapshot-id> /restored -c /app/config.yaml -p yourpass"
```

### Persisting State

Mount host directories into container to persist:

* Snapshot data and identity under `repository_path` (e.g., `./data/node1`)
* Config via bind mount.

## Utility C Tool

`tools/hashfile.c` is a small companion compiled with OpenSSL that computes SHA-256 of arbitrary files (helpful for independent verification):

Compile:

```sh
gcc -o tools/hashfile tools/hashfile.c -lcrypto
```

Usage:

```sh
./tools/hashfile /path/to/snapshot.json
```

Outputs hex digest + filename.

## Shell Helpers & Entry Point

* `scripts/bootstrap.sh`: Initializes default config and identity by briefly spinning up the agent.
* `scripts/snapshot.sh`: Wrapper to snapshot a path.
* `scripts/restore.sh`: Wrapper to restore a snapshot.
* `entrypoint.sh`: Root orchestrator that builds binaries, ensures config, launches daemon, and optionally takes a first snapshot.

Make executable:

```sh
chmod +x entrypoint.sh scripts/*.sh
```

## Example File Layout After Run

```
.
├── config.yaml
├── entrypoint.sh
├── bin/
│   ├── backup-agent
│   ├── restore-agent
│   └── peerctl
├── data/                  # repository_path
│   ├── identity.key       # persistent libp2p key
│   ├── metadata.db        # bbolt DB (snapshots, peers, blocks)
│   └── snapshots/         # encrypted snapshot metadata
├── snapshots/             # (optional local snapshot working trees)
├── .shadowvault/          # if alternate layout used
├── scripts/
│   ├── bootstrap.sh
│   ├── snapshot.sh
│   └── restore.sh
├── tools/
│   └── hashfile           # compiled C helper
└── README.md              # this document
```

## Troubleshooting & Common Issues

| Problem                         | Likely Cause                            | Remedy                                                         |
| ------------------------------- | --------------------------------------- | -------------------------------------------------------------- |
| Snapshot fails with read errors | Permissions or missing files            | Check file access, run with sufficient privileges              |
| Cannot fetch chunk from peer    | Peer offline / no announcement          | Ensure peer is connected, check gossip logs, add via `peerctl` |
| Signature validation fails      | Passphrase mismatch / tampered snapshot | Verify passphrase; reject snapshot if integrity compromised    |
| Identity changes unexpectedly   | Identity key deleted or corrupted       | Restore `identity.key` backup; avoid deleting it               |
| Peer not discovered             | DHT/bootstrap misconfig                 | Ensure bootstrap addresses are correct and reachable           |
| Cache inconsistency on restore  | Corrupted local chunk                   | Delete affected chunk and allow re-fetch from another peer     |

## Protocol Buffers

ShadowVault defines its on-wire and on-disk message formats in Protobuf, organized under `proto/`:

```

proto/
common.proto
snapshot.proto
block.proto
peer.proto
auth.proto
identity.proto
service.proto

````

### Generating Go bindings

Install the Protobuf plugins for Go:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
````

Then from the project root run:

```bash
protoc --go_out=. --go-grpc_out=. proto/*.proto
```

This will generate Go packages under `github.com/yourusername/shadowvault/proto/...`.

### File-by-file summary

* **`common.proto`**

   * `Ack` – simple acknowledgment wrapper (`ok` + `message`).
* **`snapshot.proto`**

   * `FileEntry` – path, metadata and list of chunk hashes.
   * `SnapshotMetadata` – signed snapshot descriptor (ID, parent, timestamp, files, signer, signature).
   * `SnapshotAnnouncement` – wraps the above for gossip/pubsub.
* **`block.proto`**

   * `BlockAnnounce` – tell peers “I have chunk `<hash>`”.
   * `BlockRequest` – signed request for a chunk.
   * `BlockResponse` – signed response carrying the encrypted payload.
* **`peer.proto`**

   * `PeerInfo` – `peer_id` + multiaddrs.
   * `PeerAdd` / `PeerRemove` – signed introductions or removals.
   * `PeerList` – enumeration of known peers.
* **`auth.proto`**

   * `ACL` – list of admin public keys.
   * `SignedMessage` – generic wrapper (payload + signature + pubkey).
* **`identity.proto`**

   * `Identity` – peer identity record (`peer_id` + `pubkey_base64`).
* **`service.proto`**

   * `ShadowVault` gRPC service – RPCs for snapshot announce, block request/response, peer add/remove, list peers.

## Security Considerations

* **Local passphrase**: The encryption key is derived from the passphrase; use high-entropy passphrases and protect them.
* **Identity key**: Stored unencrypted by default; restrict filesystem permissions (0600). Optionally extend to wrap with passphrase.
* **Snapshot authenticity**: Signing prevents snapshot tampering; always verify signature on restore.
* **Peer trust**: Gossip and block availability are unauthenticated unless guarded via ACL. Malicious peers could advertise bogus availability—integrity fails during fetch if data doesn't decrypt or hash mismatch occurs.
* **Replay / rollback**: Snapshot history is linear but not globally ordered; you may layer version pinning if needed.
* **Denial of Service**: A flood of bogus block requests could be mitigated by rate-limiting or proof-of-work in extensions.

## Extension Points / Developer Notes

* **Advanced chunker**: Replace simple content-defined boundary logic with full Rabin fingerprinting.
* **Remote CAS**: Overlay S3, IPFS, or other backends for wider distribution.
* **Snapshot diffing**: Visualize differences between snapshots to show added/removed chunks.
* **Gossip compression**: Batch announcements or use bloom filters to reduce chatter.
* **Access control**: Fine-grained capabilities per snapshot or time-limited tokens.
* **GUI/dashboard**: Visualize peers, snapshots, and integrity status.
* **Metric exports**: Prometheus / telemetry integration for health and sync stats.

## Contributing

1. Fork the repository.
2. Create a feature branch (e.g., `feature/remote-cas`).
3. Add or update tests demonstrating the new behavior.
4. Submit a pull request with a clear description and rationale.

Areas of high impact:

* Parallelized restore/snapshot execution.
* Peer reputation and gossip sanitization.
* Encrypted, versioned identity/key rotation.
* Plugin system for new transport/snapshot backends.

## Glossary

* **Chunk**: A small piece of a file, determined via content-defined chunking.
* **CAS**: Content-addressed storage; stores data by hash to deduplicate.
* **Snapshot**: A signed descriptor capturing a point-in-time view of directory contents via chunk hashes.
* **Peer**: Another instance of ShadowVault participating in sync.
* **PubSub**: Gossip mechanism for announcing availability.
* **ACL**: Access control list governing trusted signers/admins.
* **Identity Key**: Libp2p private key used for peer identity and signing.

## License

MIT License. See `LICENSE` for full terms.

---

Thank you for checking out ShadowVault! We hope it helps you securely back up and manage your data in a decentralized way. For any questions or contributions, please refer to the [Contributing](#contributing) section.
