package gnovm

import (
	"fmt"

	"github.com/gnolang/gno/gnovm/pkg/gnolang"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/db"
	"github.com/gnolang/gno/tm2/pkg/db/memdb"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/gnolang/gno/tm2/pkg/store/dbadapter"
)

type GnoMachineWrapper struct {
	Machine *gnolang.Machine
	Store   gnolang.Store
	DB      db.DB
	PkgPath string
}

func NewGnoMachineWrapper(pkgPath, src string) (*GnoMachineWrapper, error) {
	memDB := memdb.NewMemDB()
	baseStore := dbadapter.Store{DB: memDB}

	alloc := gnolang.NewAllocator(0)
	store := gnolang.NewStore(alloc, baseStore, baseStore)
	m := gnolang.NewMachine(pkgPath, store)

	mpkg := &std.MemPackage{
		Name: "agent",
		Path: pkgPath,
		Type: gnolang.MPUserProd,
		Files: []*std.MemFile{
			{Name: "agent.gno", Body: src},
		},
	}
	_, pv := m.RunMemPackage(mpkg, true)
	m.SetActivePackage(pv)

	return &GnoMachineWrapper{
		Machine: m,
		Store:   m.Store,
		DB:      memDB,
		PkgPath: pkgPath,
	}, nil
}

type dbEntry struct {
	K []byte
	V []byte
}

func (w *GnoMachineWrapper) ExportState() ([]byte, error) {
	it, err := w.DB.Iterator(nil, nil)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var entries []dbEntry
	for ; it.Valid(); it.Next() {
		entries = append(entries, dbEntry{K: it.Key(), V: it.Value()})
	}
	return amino.Marshal(entries)
}

func (w *GnoMachineWrapper) RestoreState(b []byte) error {
	var entries []dbEntry
	if err := amino.Unmarshal(b, &entries); err != nil {
		return err
	}

	for _, e := range entries {
		if err := w.DB.Set(e.K, e.V); err != nil {
			return err
		}
	}
	return nil
}

func (w *GnoMachineWrapper) SyncState(userInput string, unixTime int64) error {
	expr := fmt.Sprintf(`SyncState(%q, %d)`, userInput, unixTime)
	w.Machine.Eval(w.Machine.MustParseExpr(expr))
	return nil
}

func (w *GnoMachineWrapper) GetSystemContext() (string, error) {
	res := w.Machine.Eval(w.Machine.MustParseExpr("GetSystemContext()"))
	if len(res) == 0 {
		return "", fmt.Errorf("GetSystemContext returned no results")
	}
	return res[0].GetString(), nil
}

func (w *GnoMachineWrapper) AddTurn(user, agent string) error {
	expr := fmt.Sprintf(`AddTurn(%q, %q)`, user, agent)
	w.Machine.Eval(w.Machine.MustParseExpr(expr))
	return nil
}

func (w *GnoMachineWrapper) Friendship() (int, error) {
	res := w.Machine.Eval(w.Machine.MustParseExpr("self.Friendship"))
	if len(res) == 0 {
		return 0, fmt.Errorf("Friendship eval failed")
	}
	return int(res[0].GetInt()), nil
}

func (w *GnoMachineWrapper) Mood() (string, error) {
	res := w.Machine.Eval(w.Machine.MustParseExpr("currentMood.Vibe"))
	if len(res) == 0 {
		return "", fmt.Errorf("Mood eval failed")
	}
	return res[0].GetString(), nil
}
