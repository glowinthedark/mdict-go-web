.PHONY: run go-build

DICT_DIR := ~/Dictionaries
MDICT_TEMP_ASSETS_DIR := ~/.mdict/res

run: $(MDICT_TEMP_ASSETS_DIR) $(DICT_DIR) go-build
	./mdict-go/mdict-server
# 	DICT_DIR=$(DICT_DIR) MDICT_TEMP_ASSETS_DIR=$(MDICT_TEMP_ASSETS_DIR) ./mdict-go/mdict-server

$(DICT_DIR):
	mkdir -p $(DICT_DIR)

$(MDICT_TEMP_ASSETS_DIR):
	mkdir -p $(MDICT_TEMP_ASSETS_DIR)

go-build:
	cd mdict-go && go build -o mdict-server .

