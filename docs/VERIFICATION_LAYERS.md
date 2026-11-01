# The three verification layers

ShipSmoke's whole purpose is answering one question — *does what users
download actually work?* — but no single check answers it completely. Three
different layers cover three different failure classes, and v1.0.3 makes the
boundaries between them explicit.

| Layer | What it proves | What it cannot prove | Implemented by |
|---|---|---|---|
| **1. Download integrity** | The bytes you received are byte-for-byte the bytes the release published. Catches truncation, corruption, tampering, wrong asset. | Nothing about what the bytes *do*. | Action: SHA-256 against the release's `checksums.txt` before extraction (`verified: checksum` output). `shipsmoke` itself never downloads. |
| **2. Archive integrity** | The container is structurally sound: the gzip member decompresses fully and its CRC32/ISIZE trailer matches; the tar structure parses; entries are safe (no traversal, no links, within limits). | Whether an *executable's* content is the one you meant to ship (a structurally valid archive can contain a functionally broken build). | `internal/artifact` extractors. v1.0.3 closed the gap where a tar.gz whose tar structure was intact passed extraction even with corrupted gzip payload data — the member CRC is now always verified. |
| **3. Real-run verification** | The extracted entry binary starts, exits with the expected code, and produces the expected output — on the operating system it targets. | Everything above (a passing run of a corrupted-but-executable binary says nothing about integrity). | `internal/runner` + `internal/engine`. |

## Concrete failure classes and which layer catches them

| Failure | Layer 1 (checksum) | Layer 2 (archive) | Layer 3 (run) |
|---|---|---|---|
| Truncated upload | ✅ | ✅ (gzip/tar unexpected EOF) | — (never runs) |
| Flipped byte inside the gzip payload | ✅ | ✅ (member CRC mismatch — v1.0.3) | likely fails at start-up, but not guaranteed |
| Corrupted gzip trailer / CRC field | ✅ | ✅ (v1.0.3) | — |
| Trailing bytes *after* a complete gzip member | ✅ | ⚠️ accepted (documented leniency: the tar payload is complete and the member CRC verifies) | unaffected |
| structurally valid archive, functionally broken binary (missing flag, crash, wrong output) | ❌ | ❌ | ✅ |
| right binary, wrong platform packaging | ❌ | ✅ (header vs host/declaration diagnosis) | ✅ (mismatch FAIL) |

Only layers 1 + 3 together give end-to-end confidence; layer 2 sits between
them and explains *which part* of the artifact is broken. That is why the
GitHub Action performs both layer 1 (before anything executes) and layer 3
(after extraction), and why every failure report names its layer.
