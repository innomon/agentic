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

// MachineOptions contains parameters for Gno machine creation.
type MachineOptions struct {
	PkgPath string
	Store   db.DB
	Source  map[string]string // File name -> content
}

// MachineWrapper wraps a Gno machine and provides high-level operations.
type MachineWrapper struct {
	Machine *gnolang.Machine
	Store   gnolang.Store
	DB      db.DB
	PkgPath string
}

// NewMachineWrapper creates a new Gno machine wrapper.
func NewMachineWrapper(opts MachineOptions) (*MachineWrapper, error) {
	if opts.PkgPath == "" {
		opts.PkgPath = "gno.land/p/agent"
	}

	baseDB := opts.Store
	if baseDB == nil {
		baseDB = memdb.NewMemDB()
	}

	baseStore := dbadapter.Store{DB: baseDB}

	alloc := gnolang.NewAllocator(0)
	store := gnolang.NewStore(alloc, baseStore, baseStore)
	m := gnolang.NewMachine(opts.PkgPath, store)

	if len(opts.Source) > 0 {
		mpkg := &std.MemPackage{
			Name:  "main", // Usually main for execution or package name
			Path:  opts.PkgPath,
			Type:  gnolang.MPUserProd,
			Files: make([]*std.MemFile, 0, len(opts.Source)),
		}
		for name, body := range opts.Source {
			mpkg.Files = append(mpkg.Files, &std.MemFile{
				Name: name,
				Body: body,
			})
		}
		_, pv := m.RunMemPackage(mpkg, true)
		m.SetActivePackage(pv)
	}

	return &MachineWrapper{
		Machine: m,
		Store:   m.Store,
		DB:      baseDB,
		PkgPath: opts.PkgPath,
	}, nil
}

type dbEntry struct {
	K []byte
	V []byte
}

// ExportState exports the VM state as a byte slice.
func (w *MachineWrapper) ExportState() ([]byte, error) {
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

// RestoreState restores the VM state from a byte slice.
func (w *MachineWrapper) RestoreState(b []byte) error {
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

// Eval evaluates a Gno expression.
func (w *MachineWrapper) Eval(expr string) ([]gnolang.TypedValue, error) {
	parsed := w.Machine.MustParseExpr(expr)
	res := w.Machine.Eval(parsed)
	return res, nil
}

// Run executes a package main function.
func (w *MachineWrapper) Run() error {
	// Gno doesn't have a simple 'Run' for the whole machine, 
	// but we can evaluate 'main()' if it exists.
	_, err := w.Eval("main()")
	return err
}
