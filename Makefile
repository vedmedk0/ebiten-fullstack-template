.PHONY: build build-wasm build-server run clean

build: build-wasm build-server

build-wasm:
	GOOS=js GOARCH=wasm go build -o web/client.wasm ./cmd/client
	@if [ -f $$(go env GOROOT)/lib/wasm/wasm_exec.js ]; then \
		cp $$(go env GOROOT)/lib/wasm/wasm_exec.js web/wasm_exec.js; \
	elif [ -f $$(go env GOROOT)/misc/wasm/wasm_exec.js ]; then \
		cp $$(go env GOROOT)/misc/wasm/wasm_exec.js web/wasm_exec.js; \
	else \
		echo "Error: wasm_exec.js not found in Go installation"; exit 1; \
	fi

build-server:
	go build -o build/server ./cmd/server

run: build
	./build/server

clean:
	rm -rf build/server web/client.wasm
