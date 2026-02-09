package gnovm

import (
	"github.com/gnolang/gno/gnovm/pkg/gnolang"
)

type GnoMachineWrapper struct {
	Machine *gnolang.Machine
	Store   gnolang.Store
}

func NewGnoMachineWrapper(pkgPath, src string) (*GnoMachineWrapper, error) {
	alloc := gnolang.NewAllocator(0)
	store := gnolang.NewStore(alloc, nil, nil)
	m := gnolang.NewMachine(pkgPath, store)
	return &GnoMachineWrapper{Machine: m, Store: m.Store}, nil
}

func (w *GnoMachineWrapper) ExportState() ([]byte, error) {
	return nil, nil // TODO: implement state export via gnolang serialization
}

func (w *GnoMachineWrapper) RestoreState(b []byte) error {
	return nil // TODO: implement state restore via gnolang deserialization
}

func (w *GnoMachineWrapper) SyncState(userInput string, unixTime int64) error {
	return nil // TODO: evaluate agent.SyncState in GnoVM
}

func (w *GnoMachineWrapper) GetSystemContext() (string, error) {
	return "You are Gnogent, a stateful AI assistant.", nil // TODO: evaluate agent.GetSystemContext() in GnoVM
}

func (w *GnoMachineWrapper) AddTurn(user, agent string) error {
	return nil // TODO: evaluate agent.AddTurn in GnoVM
}

func (w *GnoMachineWrapper) Friendship() (int, error) {
	return 10, nil // TODO: evaluate agent.self.Friendship in GnoVM
}

func (w *GnoMachineWrapper) Mood() (string, error) {
	return "Neutral", nil // TODO: evaluate agent.currentMood.Vibe in GnoVM
}
