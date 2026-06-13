# MDict go web dictionary

## Configuration
The following default values are preset and can be overridden with env vars

| Variable               | Default                       | Description |
|-----------------------:|--------------------------------|------------|
| DICT_DIR               | ~/Dictionaries                 | folder containing `.mdx/.mdd` (scanned recursively)
| MDICT_TEMP_ASSETS_DIR  | /var/www/res                   | temporary assets store
| SERVER_IP              | 127.0.0.1                      | bind IP
| SERVER_PORT            | 8808                           | port
| SPEEXDEC               | /usr/local/bin/speexdec        | location of `speexdec` (for `.spx` audio decoding)
| NO_BROWSER             | (unset = open browser)         | set to non-empty value to prevent browser autostart

## Build
With [golang environment is installed](https://go.dev/doc/install) the following command will build and run the binary

```sh
make
```

## Run

Run compiled dictionary server:

```sh
./mdict-go/mdict-server
```

Pass custom configuration parameters via env vars:

```sh
DICT_DIR="~/path/to/custom/dictionaries" SERVER_PORT=8888 ./mdict-go/mdict-server
```

Open in browser:

- http://localhost:8808
