# MDict go web dictionary

## Configuration
Configuration parameters can be set in `config.toml` or via env vars (which take prescedemce over `config.toml`); if neither are set then default values are used.

| Param | CLI flag | Env var or `config.toml` key | Default |
|---|---|---|---|
| dict dir | `--dict-dir` | `DICT_DIR` | `~/Dictionaries` |
| asset dir | `--asset-dir` | `MDICT_TEMP_ASSETS_DIR` | `~/.mdict/res` |
| default dict | `--default-dict` | `DEFAULT_DICT` | `(none, rel. to dict-dir)` |
| server IP | `--ip` | `SERVER_IP` | `127.0.0.1` |
| server port | `--port` | `SERVER_PORT` | `8808` |
| speexdec path | `--speexdec` | `SPEEXDEC` | `/usr/bin/speexdec` |
| no browser | `--no-browser` | `NO_BROWSER=1` | `(open browser)` |
| config file | `--config` | `CONFIG_PATH` | `(auto-detect)` |

## Build
With [golang environment is installed](https://go.dev/doc/install) use the following command to build and run the `mdict-server` binary:

```sh
make
```

## Run

Run pre-compiled dictionary server:

```sh
./mdict-go/mdict-server
```

Pass custom configuration parameters via env vars:

```sh
DICT_DIR="~/path/to/custom/dictionaries" SERVER_PORT=8888 ./mdict-go/mdict-server
```

Open in browser:

- http://localhost:8808
