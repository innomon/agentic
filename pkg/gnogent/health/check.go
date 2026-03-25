package health

import (
	"github.com/innomon/agentic/pkg/gnogent/auth"
	"github.com/innomon/agentic/pkg/gnovm"

	"gorm.io/gorm"
)

type DiagnosticReport struct{ Database, GnoVM, Auth, Ready bool }

func RunDiagnostics(db *gorm.DB, vm *gnovm.MachineWrapper, pubKeyPath string) DiagnosticReport {
	r := DiagnosticReport{}

	r.Database = db.Exec("SELECT 1").Error == nil
	r.GnoVM = vm != nil
	_, err := auth.LoadPublicKey(pubKeyPath)
	r.Auth = err == nil
	r.Ready = r.Database && r.GnoVM && r.Auth

	return r
}
