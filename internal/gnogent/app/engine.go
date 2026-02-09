package app

import (
	"context"
	"fmt"
	"time"

	"github.com/innomon/agentic/internal/gnogent/auth"
	"github.com/innomon/agentic/internal/gnogent/gnovm"
	"github.com/innomon/agentic/internal/gnogent/storage"

	"gorm.io/gorm"
)

type GnoStateSummary struct {
	Friendship int
	Mood       string
	Likes      []string
}

func HandleTurn(ctx context.Context, db *gorm.DB, vm *gnovm.GnoMachineWrapper, rawToken string, userInput string) (string, GnoStateSummary, error) {
	claims, err := auth.VerifyToken(rawToken)
	if err != nil {
		return "", GnoStateSummary{}, fmt.Errorf("unauthorized: %v", err)
	}

	var session storage.AgentSession
	err = db.Where("user_id = ?", claims.UserID).First(&session).Error
	if err == nil {
		if err := vm.RestoreState(session.VMState); err != nil {
			return "", GnoStateSummary{}, fmt.Errorf("thaw failure: %v", err)
		}
	}

	now := time.Now().Unix()
	if err := vm.SyncState(userInput, now); err != nil {
		return "", GnoStateSummary{}, fmt.Errorf("gno sync error: %v", err)
	}

	systemContext, _ := vm.GetSystemContext()

	agentResponse := callLLM(systemContext, userInput)

	_ = vm.AddTurn(userInput, agentResponse)

	newStateBlob, err := vm.ExportState()
	if err != nil {
		return "", GnoStateSummary{}, fmt.Errorf("freeze failure: %v", err)
	}

	friendship, _ := vm.Friendship()
	mood, _ := vm.Mood()

	session.UserID = claims.UserID
	session.VMState = newStateBlob
	session.UpdatedAt = time.Now()
	db.Save(&session)

	return agentResponse, GnoStateSummary{
		Friendship: friendship,
		Mood:       mood,
	}, nil
}

func callLLM(systemContext, userInput string) string {
	return "TODO: LLM integration not implemented"
}
