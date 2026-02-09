//go:build tinygo.wasm

package main

import "unsafe"

//go:wasmimport env subagent_count
func subagent_count() int32

//go:wasmimport env run_subagent
func run_subagent(index int32) int32

//go:wasmimport env subagent_name
func subagent_name(index int32, buf_ptr int32, buf_cap int32) int32

//go:wasmimport env log_msg
func log_msg(ptr int32, len int32)

func main() {}

//export execute
func execute() int32 {
	n := subagent_count()
	logString("sequential-wasm: starting execution")

	for i := int32(0); i < n; i++ {
		name := getSubAgentName(i)
		logString("sequential-wasm: running sub-agent " + name)

		if rc := run_subagent(i); rc != 0 {
			logString("sequential-wasm: sub-agent " + name + " failed")
			return rc
		}
	}

	logString("sequential-wasm: all sub-agents completed successfully")
	return 0
}

func logString(msg string) {
	buf := []byte(msg)
	log_msg(int32(uintptr(unsafe.Pointer(&buf[0]))), int32(len(buf)))
}

func getSubAgentName(index int32) string {
	buf := make([]byte, 256)
	n := subagent_name(index, int32(uintptr(unsafe.Pointer(&buf[0]))), int32(len(buf)))
	if n <= 0 {
		return "unknown"
	}
	return string(buf[:n])
}

//export malloc
func wasmMalloc(size uint32) *byte {
	buf := make([]byte, size)
	return &buf[0]
}

//export free
func wasmFree(ptr *byte) {}
