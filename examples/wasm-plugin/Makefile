WASM_OUT := plugin.wasm

.PHONY: build clean

build:
	tinygo build -o $(WASM_OUT) -target wasi -buildmode c-shared ./main.go

clean:
	rm -f $(WASM_OUT)
