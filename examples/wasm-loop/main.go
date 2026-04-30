//go:build tinygo.wasm

package main

import (
	"strings"
	"unsafe"
)

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

//go:wasmimport env get_input_len
func get_input_len() int32

//go:wasmimport env get_input
func get_input(buf_ptr int32, buf_cap int32) int32

//go:wasmimport env set_input
func set_input(ptr int32, len int32)

//go:wasmimport env log_msg
func log_msg(ptr int32, len int32)

func main() {}

const maxLoops = 3

//export execute
func execute() int32 {
	n := subagent_count()
	if n < 2 {
		logString("loop-wasm: error: at least 2 sub-agents (Worker, Critic) required")
		return 1
	}

	initialInput := getInitialInput()
	logString("loop-wasm: starting refinement loop for: " + initialInput)

	currentInput := initialInput

	for i := 0; i < maxLoops; i++ {
		logString("loop-wasm: iteration " + itoa(int32(i+1)))

		// 1. Run the Worker (Agent 0)
		logString("loop-wasm: running worker...")
		setInput(currentInput)
		if rc := run_subagent(0); rc != 0 {
			return rc
		}
		workerOutput := getSubAgentOutput(0)

		// 2. Run the Critic (Agent 1)
		logString("loop-wasm: running critic...")
		criticPrompt := "Worker Output:\n" + workerOutput + "\n\nIs this output satisfactory? If yes, start your response with '[APPROVED]'. If no, provide feedback for refinement."
		setInput(criticPrompt)
		if rc := run_subagent(1); rc != 0 {
			return rc
		}
		criticOutput := getSubAgentOutput(1)

		// 3. Check for stop criteria
		if strings.Contains(criticOutput, "[APPROVED]") {
			logString("loop-wasm: critic approved output. stopping.")
			return 0
		}

		logString("loop-wasm: critic requested refinement. looping back.")
		// Prepare input for next worker run: original task + feedback
		currentInput = "Original Task: " + initialInput + "\n\nPrevious Attempt: " + workerOutput + "\n\nCritic Feedback: " + criticOutput + "\n\nPlease refine the output based on this feedback."
	}

	logString("loop-wasm: reached max loops. stopping.")
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

func getInitialInput() string {
	length := get_input_len()
	if length <= 0 {
		return ""
	}
	buf := make([]byte, length)
	n := get_input(int32(uintptr(unsafe.Pointer(&buf[0]))), int32(len(buf)))
	return string(buf[:n])
}

func getSubAgentOutput(index int32) string {
	length := subagent_output_len(index)
	if length <= 0 {
		return ""
	}
	buf := make([]byte, length)
	n := subagent_output_get(index, int32(uintptr(unsafe.Pointer(&buf[0]))), int32(len(buf)))
	return string(buf[:n])
}

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
