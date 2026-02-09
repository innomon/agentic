WASM_OUT := sequential.wasm

.PHONY: build clean

build:
	tinygo build -o $(WASM_OUT) -scheduler=none --no-debug -target wasi ./main.go

clean:
	rm -f $(WASM_OUT)
