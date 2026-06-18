.PHONY: build run clean clean-go-global-module-cache

DICT_DIR := $(HOME)/Dictionaries
MDICT_TEMP_ASSETS_DIR := $(HOME)/.mdict/res
SERVER_IP := 0.0.0.0
SERVER_PORT := 8808
SPEEXDEC := $(shell which speexdec 2>/dev/null)
DEFAULT_DICT := ldoce6.mdx

SOURCES := $(wildcard *.go) $(wildcard web/*) go.mod go.sum

build: mdict-go-web

mdict-go-web: $(SOURCES)
	go build -o mdict-go-web .

run: $(MDICT_TEMP_ASSETS_DIR) $(DICT_DIR) build
	./mdict-go-web \
	    --dict-dir $(DICT_DIR) \
	    --asset-dir $(MDICT_TEMP_ASSETS_DIR) \
	    --default-dict $(DEFAULT_DICT) \
	    --ip $(SERVER_IP) \
	    --port $(SERVER_PORT)

$(DICT_DIR):
	mkdir -p $@

$(MDICT_TEMP_ASSETS_DIR):
	mkdir -p $@

clean:
	rm -f mdict-go-web

clean-go-global-module-cache:
	go clean -modcache
