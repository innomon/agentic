// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/artifact"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/server/adkrest/internal/models"
	"google.golang.org/adk/session"
)

// RuntimeAPIController is the controller for the Runtime API.
type RuntimeAPIController struct {
	sessionService  session.Service
	artifactService artifact.Service
	agentLoader     agent.Loader
}

// NewRuntimeAPIController creates the controller for the Runtime API.
func NewRuntimeAPIController(sessionService session.Service, agentLoader agent.Loader, artifactService artifact.Service) *RuntimeAPIController {
	return &RuntimeAPIController{sessionService: sessionService, agentLoader: agentLoader, artifactService: artifactService}
}

// RunAgent executes a non-streaming agent run for a given session and message.
func (c *RuntimeAPIController) RunHandler(rw http.ResponseWriter, req *http.Request) error {
	runAgentRequest, err := decodeRequestBody(req)
	if err != nil {
		return err
	}
	sessionEvents, err := c.runAgent(req.Context(), runAgentRequest)
	if err != nil {
		return err
	}
	var events []models.Event
	for _, event := range sessionEvents {
		events = append(events, models.FromSessionEvent(*event))
	}
	EncodeJSONResponse(events, http.StatusOK, rw)
	return nil
}

// RunAgent executes a non-streaming agent run for a given session and message.
func (c *RuntimeAPIController) runAgent(ctx context.Context, runAgentRequest models.RunAgentRequest) ([]*session.Event, error) {
	err := c.validateSessionExists(ctx, runAgentRequest.AppName, runAgentRequest.UserId, runAgentRequest.SessionId)
	if err != nil {
		return nil, err
	}

	r, rCfg, err := c.getRunner(runAgentRequest)
	if err != nil {
		return nil, err
	}

	resp := r.Run(ctx, runAgentRequest.UserId, runAgentRequest.SessionId, &runAgentRequest.NewMessage, *rCfg)

	var events []*session.Event
	for event, err := range resp {
		if err != nil {
			return nil, newStatusError(fmt.Errorf("run agent: %w", err), http.StatusInternalServerError)
		}
		events = append(events, event)
	}
	return events, nil
}

// RunSSEHandler executes an agent run and streams the resulting events using Server-Sent Events (SSE).
// PATCHED: Removed explicit WriteHeader call and fixed error event format
func (c *RuntimeAPIController) RunSSEHandler(rw http.ResponseWriter, req *http.Request) error {
	flusher, ok := rw.(http.Flusher)
	if !ok {
		return newStatusError(fmt.Errorf("streaming not supported"), http.StatusInternalServerError)
	}

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")

	runAgentRequest, err := decodeRequestBody(req)
	if err != nil {
		return err
	}

	err = c.validateSessionExists(req.Context(), runAgentRequest.AppName, runAgentRequest.UserId, runAgentRequest.SessionId)
	if err != nil {
		return err
	}

	r, rCfg, err := c.getRunner(runAgentRequest)
	if err != nil {
		return err
	}

	resp := r.Run(req.Context(), runAgentRequest.UserId, runAgentRequest.SessionId, &runAgentRequest.NewMessage, *rCfg)

	// PATCHED: Track if we've started streaming (sent first event)
	// This helps us decide whether we can still return an error to NewErrorHandler
	streamStarted := false

	// PATCHED: Removed explicit rw.WriteHeader(http.StatusOK)
	// The first write will implicitly set 200 OK
	for event, err := range resp {
		if err != nil {
			// PATCHED: Send error as proper SSE event with JSON payload
			errorEvent := models.Event{
				ID:           uuid.NewString(),
				Time:         time.Now().Unix(),
				Author:       "system",
				ErrorCode:    "EXECUTION_ERROR",
				ErrorMessage: err.Error(),
			}
			if flashErr := flashEvent(flusher, rw, errorEvent); flashErr != nil {
				// If we haven't started streaming yet, we can return the error
				// which will be handled by NewErrorHandler
				if !streamStarted {
					return flashErr
				}
				// Otherwise, just continue (can't change status code now)
				continue
			}
			streamStarted = true
			continue
		}
		err = flashEvent(flusher, rw, models.FromSessionEvent(*event))
		if err != nil {
			if !streamStarted {
				return err
			}
			continue
		}
		streamStarted = true
	}
	return nil
}

// PATCHED: Accept models.Event directly to support both session events and synthetic error events
func flashEvent(flusher http.Flusher, rw http.ResponseWriter, event models.Event) error {
	_, err := fmt.Fprintf(rw, "data: ")
	if err != nil {
		return newStatusError(fmt.Errorf("write response: %w", err), http.StatusInternalServerError)
	}
	err = json.NewEncoder(rw).Encode(event)
	if err != nil {
		return newStatusError(fmt.Errorf("encode response: %w", err), http.StatusInternalServerError)
	}
	_, err = fmt.Fprintf(rw, "\n")
	if err != nil {
		return newStatusError(fmt.Errorf("write response: %w", err), http.StatusInternalServerError)
	}
	flusher.Flush()
	return nil
}

func (c *RuntimeAPIController) validateSessionExists(ctx context.Context, appName, userID, sessionID string) error {
	_, err := c.sessionService.Get(ctx, &session.GetRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		return newStatusError(fmt.Errorf("get session: %w", err), http.StatusNotFound)
	}
	return nil
}

func (c *RuntimeAPIController) getRunner(req models.RunAgentRequest) (*runner.Runner, *agent.RunConfig, error) {
	curAgent, err := c.agentLoader.LoadAgent(req.AppName)
	if err != nil {
		return nil, nil, newStatusError(fmt.Errorf("load agent: %w", err), http.StatusInternalServerError)
	}

	r, err := runner.New(runner.Config{
		AppName:         req.AppName,
		Agent:           curAgent,
		SessionService:  c.sessionService,
		ArtifactService: c.artifactService,
	},
	)
	if err != nil {
		return nil, nil, newStatusError(fmt.Errorf("create runner: %w", err), http.StatusInternalServerError)
	}

	streamingMode := agent.StreamingModeNone
	if req.Streaming {
		streamingMode = agent.StreamingModeSSE
	}
	return r, &agent.RunConfig{
		StreamingMode: streamingMode,
	}, nil
}

func decodeRequestBody(req *http.Request) (decodedReq models.RunAgentRequest, err error) {
	var runAgentRequest models.RunAgentRequest
	defer func() {
		err = req.Body.Close()
	}()
	d := json.NewDecoder(req.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&runAgentRequest); err != nil {
		return runAgentRequest, newStatusError(fmt.Errorf("decode request: %w", err), http.StatusBadRequest)
	}
	return runAgentRequest, nil
}
