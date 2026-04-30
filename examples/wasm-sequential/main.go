//go:build tinygo.wasm

package main

import "unsafe"

//go:wasmimport env subagent_count
func subagent_count() int32

//go:wasmimport env run_subagent
func run_subagent(index int32) int32

//go:wasmimport env subagent_name
func subagent_name(index int32, buf_ptr int32, buf_cap int32) int32

//go:wasmimport env subagent_output_len
func subagent_output_len(index int32) int32

//go:wasmimport env subagent_output_get
func subagent_output_get(index int32, buf_ptr int32, buf_cap int32) int32

//go:wasmimport env set_input
func set_input(ptr int32, len int32)

//go:wasmimport env log_msg
func log_msg(ptr int32, len int32)

func main() {}

//export execute
func execute() int32 {
	n := subagent_count()
	logString("sequential-wasm: starting execution with chaining support")

	for i := int32(0); i < n; i++ {
		name := getSubAgentName(i)
		logString("sequential-wasm: running sub-agent " + name)

		if rc := run_subagent(i); rc != 0 {
			logString("sequential-wasm: sub-agent " + name + " failed")
			return rc
		}

		// Improvement: Capture output and chain it to the next agent if available
		output := getSubAgentOutput(i)
		if output != "" {
			logString("sequential-wasm: captured output from " + name + " (length: " + itoa(int32(len(output))) + ")")

			// If there's another agent, feed this output as its input
			if i+1 < n {
				logString("sequential-wasm: chaining output to next agent")
				setInput(output)
			}
		}
	}

	logString("sequential-wasm: all sub-agents completed successfully")
	return 0
}

func logString(msg string) {
	buf := []byte(msg)
	log_msg(int32(uintptr(unsafe.Pointer(&buf[0]))), int32(len(buf)))
}

func setInput(input string) {
	buf := []byte(input)
	set_input(int32(uintptr(unsafe.Pointer(&buf[0]))), int32(len(buf)))
}

func getSubAgentName(index int32) string {
	buf := make([]byte, 256)
	n := subagent_name(index, int32(uintptr(unsafe.Pointer(&buf[0]))), int32(len(buf)))
	if n <= 0 {
		return "unknown"
	}
	return string(buf[:n])
}

func getSubAgentOutput(index int32) string {
	length := subagent_output_len(index)
	if length <= 0 {
		return ""
	}

	buf := make([]byte, length)
	n := subagent_output_get(index, int32(uintptr(unsafe.Pointer(&buf[0]))), int32(len(buf)))
	if n <= 0 {
		return ""
	}
	return string(buf[:n])
}

// Simple itoa for logging since we are in tinygo
func itoa(n int32) string {
	if n == 0 {
		return "0"
	}
	var res []byte
	for n > 0 {
		res = append([]byte{byte('0' + (n % 10))}, res...)
		n /= 10
	}
	return string(res)
}

//export malloc
func wasmMalloc(size uint32) *byte {
	buf := make([]byte, size)
	return &buf[0]
}

//export free
func wasmFree(ptr *byte) {}
