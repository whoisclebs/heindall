# Local pprof workflow

pprof is disabled by default. Enable it only for local investigation:

```bash
PPROF_ENABLED=true docker compose up
```

The compose file does not publish the API worker port by default, so for pprof you
should either temporarily publish a worker port, `docker exec` into a container,
or run the API binary directly outside the challenge submission path.

Useful endpoints once you can reach a worker listener:

- `http://localhost:8080/debug/pprof/`
- `http://localhost:8080/debug/pprof/profile?seconds=30`
- `http://localhost:8080/debug/pprof/heap`
- `http://localhost:8080/debug/pprof/goroutine?debug=1`

Example capture flow during load testing:

```bash
# CPU profile while k6 or another load generator is running
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# Heap profile snapshot
go tool pprof http://localhost:8080/debug/pprof/heap

# Goroutine dump
curl "http://localhost:8080/debug/pprof/goroutine?debug=1"
```

Use this only outside competition submissions. Keep `PPROF_ENABLED=false` for normal challenge builds.
