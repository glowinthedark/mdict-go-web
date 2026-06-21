# Project Overview: mdict-go-web

## Summary
mdict-go-web is a self-contained HTTP dictionary server written in Go for serving MDict (`.mdx` / `.mdd`) dictionary files through a browser interface. It runs as a single binary and embeds its frontend assets, requiring no external runtime dependencies beyond optional tools like `speexdec` for audio decoding.

The application acts as both backend and frontend:
- Backend: Go HTTP server that parses dictionary files and serves search results and assets
- Frontend: Embedded HTML/JS UI (`web/mdict.html`, `mark.min.js`)

Default access: http://localhost:8808

---

## Architecture

### 1. Entry Point
- `main.go`
  - Handles CLI flags, environment variables, and config file loading
  - Resolves configuration priority: CLI > env > config.toml > defaults
  - Starts HTTP server
  - Embeds frontend assets via `go:embed`
  - Maintains in-memory cache of dictionary readers

### 2. Configuration System
- `config.go`
  - Loads TOML config
  - Provides fallback lookup via `getConf`

- Config sources:
  - CLI flags
  - Environment variables
  - `config.toml`

- Key parameters:
  - Dictionary directory
  - Asset cache directory
  - Default dictionary
  - Server IP/port
  - Speex decoder path

### 3. Dictionary Engine
- `reader.go`
  - High-level abstraction for dictionary access
  - Responsible for opening `.mdx` files and querying entries

- `internal/gomdict/`
  - Core parsing logic for MDict format
  - Includes:
    - `mdict.go`, `mdict_base.go`, `mdict_def.go`
    - `mdict_accessor.go` (data access)
    - `mdict_record_range_tree.go` (indexing/search optimization)
    - `xml_dictionary.go` (XML-based entries)
    - `util.go` (helpers)
  - Handles decompression, indexing, and record lookup

### 4. HTTP Layer
- Implemented in `main.go`
  - Serves:
    - Search queries (`?q=`)
    - Static frontend
    - Extracted `.mdd` assets
  - Uses Go standard `net/http`

### 5. Frontend
- `web/mdict.html`
  - Main UI
  - Handles user input and renders results

- `web/mark.min.js`
  - Client-side highlighting of search terms

### 6. Asset Handling
- `.mdd` resources extracted on demand
- Cached in `assetRoot`
- Served via HTTP once extracted

### 7. Caching
- Global in-memory cache:
  - `map[string]*Reader`
  - Prevents repeated parsing of dictionaries
  - Protected by `sync.Mutex`

---

## Execution Flow

1. Parse CLI flags
2. Load configuration (file + env)
3. Resolve effective settings
4. Initialize server
5. On request:
   - Load dictionary (cached)
   - Execute search or asset lookup
   - Return HTML response

---

## Build & Run

- Build:
  - `make`

- Run:
  - `./mdict-go-web`

- Optional flags:
  - `--dict-dir`, `--port`, `--ip`, etc.

---

## Notable Design Choices

- Single-binary deployment (no external services)
- Embedded frontend (no separate build pipeline)
- Lazy loading + caching of dictionaries
- Minimal dependencies (mostly Go standard library)

---

## Strengths

- Simple deployment model
- Fast lookup due to in-memory caching
- Clean separation between parsing logic and server layer
- Flexible configuration system

---

## Limitations / Risks

- Global cache uses a single mutex (potential contention under load)
- No explicit cache eviction strategy
- Limited observability (no metrics/log structuring)
- Tight coupling of HTTP layer and application logic in `main.go`
- Frontend is static and minimally structured

---

## Potential Improvements

- Replace global mutex with RWMutex or sharded cache
- Add LRU/size-based cache eviction
- Separate HTTP handlers into dedicated package
- Add API endpoints (JSON) for external clients
- Improve frontend modularity
- Add structured logging and metrics

---

## Repository Structure (High-Level)

- `main.go` — application entry and HTTP server
- `reader.go` — dictionary abstraction
- `config.go` — configuration handling
- `internal/gomdict/` — MDict parsing engine
- `web/` — embedded frontend assets
- `notes/` — design notes and experiments
- `Makefile` — build/run commands

---

## Conclusion

This project is a compact, production-capable local dictionary server optimized for simplicity and portability. Its strongest aspect is the tight integration of parsing, serving, and UI into a single binary, making it easy to distribute and run. The main trade-offs are around scalability, modularity, and observability, which are acceptable given its local-first design goals.
