# MDict go web dictionary

## Configuration
Configuration parameters can be set in `config.toml` or via env vars (which take prescedemce over `config.toml`); if neither are set then default values are used.

| Variable               | Default                       | Description |
|-----------------------:|--------------------------------|------------|
| DICT_DIR               | ~/Dictionaries                 | folder containing `.mdx/.mdd` (scanned recursively)
| DEFAULT_DICT           |                                | relative path to the default dictionary
| MDICT_TEMP_ASSETS_DIR  | ~/.mdict/res                   | temporary assets store
| SERVER_IP              | 127.0.0.1                      | bind IP
| SERVER_PORT            | 8808                           | port
| SPEEXDEC               | /usr/local/bin/speexdec        | location of `speexdec` (for `.spx` audio decoding)
| NO_BROWSER             | (unset = open browser)         | set to non-empty value to prevent browser autostart

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
