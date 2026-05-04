<a id="readme-top"></a>

# Heindall

High-performance backend implementation for [Rinha de Backend 2026](https://github.com/zanfranceschi/rinha-de-backend-2026).

## About The Project

Heindall is a fraud-detection backend built for the Rinha de Backend 2026 challenge.

The service receives transaction payloads, converts them into the official 14-dimensional fraud-detection vector, searches the official reference dataset for the 5 nearest neighbors, and returns the challenge response:

```json
{
  "approved": true,
  "fraud_score": 0.0
}
```

### Built With

- [Go](https://go.dev/)
- [Golpher](https://github.com/go-golpher/golpher)
- [Rust](https://www.rust-lang.org/)
- Docker Compose
- GitHub Packages / GHCR

## Repository Layout

```text
apps/
  api/                Go API and preprocessing tools
    cmd/api/          API entrypoint
    cmd/preprocess/   Dataset-to-index preprocessor
    internal/         App wiring, router, fraud domain and vector search
  load-balancer/      Rust round-robin reverse proxy
data/                 Official dataset and generated binary index
deploy/               Dockerfiles
docs/                 Architecture notes
scripts/              Local helper scripts
```

## Getting Started

### Prerequisites

- Go 1.23.6+
- Rust toolchain
- Docker and Docker Compose
- Git LFS

### Installation

Clone the repository and fetch LFS objects:

```bash
git clone https://github.com/whoisclebs/heindall.git
cd heindall
git lfs pull
git submodule update --init --recursive
```

Download the official challenge datasets if needed:

```bash
sh scripts/download-datasets.sh
```

Generate the compact binary index:

```bash
sh scripts/preprocess.sh
```

## Usage

Run the Go API locally:

```bash
cd apps/api
INDEX_PATH=../../data/index.heindall.bin go run ./cmd/api
```

Run the full challenge topology:

```bash
docker compose up --build
```

The load balancer exposes the challenge port:

```text
http://localhost:9999
```

Required endpoints:

- `GET /ready`
- `POST /fraud-score`

## Development

Run API tests:

```bash
cd apps/api
go test ./...
```

Run load balancer tests:

```bash
cargo test --manifest-path apps/load-balancer/Cargo.toml
```

Run the vector search benchmark:

```bash
cd apps/api
go test ./internal/fraud -bench=. -benchmem -run '^$'
```

Run the official k6 smoke and load tests from the challenge specs submodule:

```bash
sh scripts/bench.sh
```

The script expects the stack to be available at `http://localhost:9999`.

Validate Compose configuration:

```bash
docker compose config
```

## Architecture

- `apps/api` contains the full Go application module.
- `apps/load-balancer` contains the Rust round-robin reverse proxy.
- `specs` is a Git submodule pointing to the official challenge repository.
- `data/references.json.gz` is preprocessed into `data/index.heindall.bin`.
- Runtime services are defined in `docker-compose.yml` with one load balancer and two API instances.

See [docs/architecture.md](docs/architecture.md) for more details.

## Challenge References

- [Official repository](https://github.com/zanfranceschi/rinha-de-backend-2026)
- [API documentation](https://github.com/zanfranceschi/rinha-de-backend-2026/blob/main/docs/en/API.md)
- [Detection rules](https://github.com/zanfranceschi/rinha-de-backend-2026/blob/main/docs/en/DETECTION_RULES.md)
- [Dataset documentation](https://github.com/zanfranceschi/rinha-de-backend-2026/blob/main/docs/en/DATASET.md)

## Container Images

The GitHub Actions workflow publishes:

- `ghcr.io/whoisclebs/heindall-api:latest`
- `ghcr.io/whoisclebs/heindall-lb:latest`

## License

Distributed under the repository license. See `LICENSE` if present.

<p align="right">(<a href="#readme-top">back to top</a>)</p>
