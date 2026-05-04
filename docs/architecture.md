# Architecture Notes

## Official constraints

- Expose the load balancer on port `9999`.
- At least one load balancer and two API instances.
- Load balancer must only forward requests and must not inspect fraud payloads.
- Public `linux-amd64` images.
- Total declared limits: max `1 CPU` and `350MB` memory.
- Docker network mode must remain `bridge`; no `host` and no `privileged`.

## Baseline topology

```text
client -> Rust round-robin load balancer :9999 -> api1/api2
```

Runtime applications live under `apps/`:

- `apps/api`: Go/Golpher fraud-detection API, including all Go application code and its `internal/` packages.
- `apps/load-balancer`: Rust round-robin load balancer.

The Go module file lives with the Go application at `apps/api/go.mod`. The repository root is a monorepo root, not the Go module root. The module depends on the published Golpher version; if local Golpher changes are needed, use a local `go work` during development instead of committing a `replace` into the application module.

The compliant topology uses a deliberately dumb Rust proxy with simple round-robin because the challenge text explicitly says “simple round-robin”. Least-load is out of the official path unless explicitly allowed.

## Runtime strategy

Official datasets live in the Rinha repository under `resources/` and can be downloaded locally with `scripts/download-datasets.sh`.

1. Load the embedded IVF v2 index generated from `references.json.gz`.
2. Vectorize each request into 14 dimensions.
3. Query the IVF search engine that returns the 5 nearest labels.
4. Return `approved = fraud_score < 0.6`.

## Performance roadmap

1. Correct baseline: exact KNN over small datasets for tests.
2. Quantized storage: store vectors as compact `int16` instead of JSON/float objects.
3. IVF v2 index: cluster references into centroid lists, scan top probes, and use bounded repair for ambiguous cases.
4. Hot path tuning: benchmark distance calculation; consider Go assembly/SIMD only if it beats compiler-generated code with measurable impact.
