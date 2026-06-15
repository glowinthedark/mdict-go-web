.PHONY: run go-build clean

DICT_DIR := ~/Dictionaries
MDICT_TEMP_ASSETS_DIR := ~/.mdict/res

run: $(MDICT_TEMP_ASSETS_DIR) $(DICT_DIR) go-build
	./mdict-server
# 	DICT_DIR=$(DICT_DIR) MDICT_TEMP_ASSETS_DIR=$(MDICT_TEMP_ASSETS_DIR) ./mdict-server

$(DICT_DIR):
	mkdir -p $(DICT_DIR)

$(MDICT_TEMP_ASSETS_DIR):
	mkdir -p $(MDICT_TEMP_ASSETS_DIR)

go-build:
	go build -o mdict-server .

clean:
	go clean -modcache
