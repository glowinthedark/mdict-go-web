# MDict go web dictionary
Fast, zero-dependency MDict dictionary that runs in your browser (all-in-one-client-server) on [http://localhost:8808](http://localhost:8808).

## Download
Get the version for your OS from [**releases**](https://github.com/glowinthedark/mdict-go-web/releases) - save the binary; `chmod +x mdict-server` and then run it in the terminal. For help use `mdict-server -h`.

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
./mdict-server
```

Pass custom configuration parameters via env vars:

```sh
DICT_DIR="~/path/to/custom/dictionaries" SERVER_PORT=8888 ./mdict-server
```

Open in browser:

- http://localhost:8808

## Help

```sh
mdict-server --help                               
mdict-server — MDict (.mdx/.mdd) HTTP dictionary server

USAGE
  mdict-server [flags]

FLAGS
  --config       <path>   Path to config.toml (overrides auto-detect)
                          env: CONFIG_PATH

  --dict-dir     <path>   Directory with .mdx/.mdd dictionary files
                          env: DICT_DIR              toml: DICT_DIR
                          default: ~/Dictionaries

  --asset-dir    <path>   Cache dir for extracted .mdd resources
                          env: MDICT_TEMP_ASSETS_DIR  toml: MDICT_TEMP_ASSETS_DIR
                          default: ~/.mdict/res

  --default-dict <rel>    Default dictionary, relative to dict-dir
                          e.g. "en/Oxford.mdx"
                          env: DEFAULT_DICT           toml: DEFAULT_DICT

  --ip           <addr>   Listen IP address
                          env: SERVER_IP              toml: SERVER_IP
                          default: 127.0.0.1

  --port         <port>   Listen port
                          env: SERVER_PORT            toml: SERVER_PORT
                          default: 8808

  --speexdec     <path>   Path to speexdec binary (Speex audio decoding)
                          env: SPEEXDEC               toml: SPEEXDEC
                          default: /usr/local/bin/speexdec

  --no-browser            Do not open a browser tab on startup
                          env/toml: NO_BROWSER=1

  -h, --help              Show this help and exit

CONFIG FILE SEARCH ORDER
  1. --config flag / CONFIG_PATH env var
  2. <executable-dir>/config.toml
  3. ~/.mdict/config.toml
  4. /etc/mdict/config.toml
  5. ./config.toml

PRIORITY (highest → lowest)
  CLI flag  >  environment variable  >  config.toml  >  built-in default

EXAMPLE config.toml
  DICT_DIR              = "/data/dicts"
  MDICT_TEMP_ASSETS_DIR = "/tmp/mdict-res"
  DEFAULT_DICT          = "en/Oxford.mdx"
  SERVER_IP             = "0.0.0.0"
  SERVER_PORT           = "9000"
  SPEEXDEC              = "/usr/bin/speexdec"
  NO_BROWSER            = "1"

EXAMPLES
  mdict-server
  mdict-server --dict-dir ~/Books/Dicts --port 9090 --no-browser
  mdict-server --config /etc/mdict/config.toml --default-dict "en/Oxford.mdx"
  SERVER_PORT=9000 mdict-server
```
