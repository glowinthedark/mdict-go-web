.PHONY: run go-build clean

DICT_DIR := ~/Dictionaries
MDICT_TEMP_ASSETS_DIR := ~/.mdict/res

run: $(MDICT_TEMP_ASSETS_DIR) $(DICT_DIR) go-build
	./mdict-go-web
# 	DICT_DIR=$(DICT_DIR) MDICT_TEMP_ASSETS_DIR=$(MDICT_TEMP_ASSETS_DIR) ./mdict-go-web

$(DICT_DIR):
	mkdir -p $(DICT_DIR)

$(MDICT_TEMP_ASSETS_DIR):
	mkdir -p $(MDICT_TEMP_ASSETS_DIR)

go-build:
	go build -o mdict-go-web .

clean:
	go clean -modcache
	rm mdict-go-web
