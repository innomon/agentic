package gnovm

import (
	"fmt"
)

// AgentWrapper provides agent-specific methods on top of a Gno machine.
type AgentWrapper struct {
	*MachineWrapper
}

// NewAgentWrapper creates a new agent wrapper.
func NewAgentWrapper(pkgPath, src string) (*AgentWrapper, error) {
	wrapper, err := NewMachineWrapper(MachineOptions{
		PkgPath: pkgPath,
		Source: map[string]string{
			"agent.gno": src,
		},
	})
	if err != nil {
		return nil, err
	}
	return &AgentWrapper{MachineWrapper: wrapper}, nil
}

// SetInput sets the input variable in the Gno package.
func (w *AgentWrapper) SetInput(input string) error {
	expr := fmt.Sprintf(`Input = %q`, input)
	_, err := w.Eval(expr)
	return err
}

// GetOutput retrieves the output variable from the Gno package.
func (w *AgentWrapper) GetOutput() (string, error) {
	res, err := w.Eval("Output")
	if err != nil || len(res) == 0 {
		return "", fmt.Errorf("GetOutput failed: %v", err)
	}
	return res[0].String(), nil
}

// SyncState updates the agent state with user input and current time.
func (w *AgentWrapper) SyncState(userInput string, unixTime int64) error {
	expr := fmt.Sprintf(`SyncState(%q, %d)`, userInput, unixTime)
	_, err := w.Eval(expr)
	return err
}

// GetSystemContext retrieves the system context from the Gno brain.
func (w *AgentWrapper) GetSystemContext() (string, error) {
	res, err := w.Eval("GetSystemContext()")
	if err != nil || len(res) == 0 {
		return "", fmt.Errorf("GetSystemContext failed: %v", err)
	}
	return res[0].String(), nil
}

// AddTurn adds a conversation turn to the Gno brain's history.
func (w *AgentWrapper) AddTurn(user, agent string) error {
	expr := fmt.Sprintf(`AddTurn(%q, %q)`, user, agent)
	_, err := w.Eval(expr)
	return err
}
