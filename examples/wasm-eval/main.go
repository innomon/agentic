//go:build tinygo.wasm

package main

import (
	"fmt"
	"strings"
	"unsafe"
)

// Host functions
//go:wasmimport env subagent_count
func subagent_count() int32

//go:wasmimport env subagent_name
func subagent_name(index int32, buf_ptr int32, buf_cap int32) int32

//go:wasmimport env run_subagent
func run_subagent(index int32) int32

//go:wasmimport env subagent_output_len
func subagent_output_len(index int32) int32

//go:wasmimport env subagent_output_get
func subagent_output_get(index int32, buf_ptr int32, buf_cap int32) int32

//go:wasmimport env subagent_duration_ms
func subagent_duration_ms(index int32) int64

//go:wasmimport env subagent_token_input
func subagent_token_input(index int32) int32

//go:wasmimport env subagent_token_output
func subagent_token_output(index int32) int32

//go:wasmimport env set_input
func set_input(ptr int32, len int32)

//go:wasmimport env get_input_len
func get_input_len() int32

//go:wasmimport env get_input
func get_input(buf_ptr int32, buf_cap int32) int32

//go:wasmimport env log_msg
func log_msg(ptr int32, len int32)

func main() {}

//export execute
func execute() int32 {
	input := getInitialInput()
	logString("EvalOrchestrator: starting evaluation for task: " + input)

	count := subagent_count()
	if count < 2 {
		logString("Error: Need at least 2 sub-agents (Candidates... and one Judge)")
		return 1
	}

	judgeIndex := count - 1
	judgeName := getSubagentName(judgeIndex)
	logString("Using judge: " + judgeName)

	var report strings.Builder
	report.WriteString("# Evaluation Report\n\n")
	report.WriteString("| Candidate | Latency (ms) | Tokens (In/Out) | Score & Justification |\n")
	report.WriteString("| :--- | :--- | :--- | :--- |\n")

	for i := int32(0); i < judgeIndex; i++ {
		name := getSubagentName(i)
		logString("Evaluating candidate " + itoa(i) + ": " + name)

		// Run candidate with original input
		setInput(input)
		if run_subagent(i) != 0 {
			logString("Error running sub-agent " + itoa(i))
			return 1
		}

		output := getSubAgentOutput(i)
		duration := subagent_duration_ms(i)
		tIn := subagent_token_input(i)
		tOut := subagent_token_output(i)

		logString("Candidate " + name + " finished in " + itoa(int32(duration)) + "ms with " + itoa(tIn) + "/" + itoa(tOut) + " tokens")

		// Now call the judge to score this output
		judgePrompt := "TASK: " + input + "\n\nRESPONSE TO SCORE:\n" + output + "\n\nSCORING INSTRUCTIONS:\nProvide a score from 1-10 and a brief justification."
		setInput(judgePrompt)
		if run_subagent(judgeIndex) != 0 {
			logString("Error running judge agent")
			return 1
		}

		judgeOutput := getSubAgentOutput(judgeIndex)
		report.WriteString("| " + name + " | " + itoa(int32(duration)) + " | " + itoa(tIn) + "/" + itoa(tOut) + " | " + judgeOutput + " |\n")
	}

	logString("Evaluation complete.")
	fmt.Println(report.String())
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

func getSubagentName(index int32) string {
	buf := make([]byte, 64)
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
	return string(buf[:n])
}

func itoa(n int32) string {
	if n == 0 {
		return "0"
	}
	var res []byte
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		res = append([]byte{byte('0' + (n % 10))}, res...)
		n /= 10
	}
	if neg {
		res = append([]byte{'-'}, res...)
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
