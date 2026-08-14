package registry

import (
	"strings"
	"testing"
)

func TestWorkflowGraphConfigAndExport(t *testing.T) {
	cfg := &WorkflowAgentConfig{
		AgentBase: AgentBase{
			Type:        "workflow",
			Description: "Test cyclic graph workflow",
		},
		Nodes: []WorkflowNodeEntry{
			{
				Name:        "drafter",
				Agent:       "DraftAgent",
				InputMap:    map[string]string{"input": "user_input"},
				OutputMap:   map[string]string{"output": "draft_output"},
				Retries:     2,
			},
			{
				Name:        "evaluator",
				Agent:       "EvalAgent",
			},
			{
				Name:        "refinement",
				Agent:       "RefineAgent",
			},
			{
				Name:        "sub_flow",
				SubWorkflow: "NestedWorkflow",
			},
		},
		Edges: []WorkflowEdgeEntry{
			{From: "START", To: "drafter"},
			{From: "drafter", To: "evaluator"},
			{From: "evaluator", To: "refinement", Route: "needs_revision"},
			{From: "refinement", To: "evaluator"},
			{From: "evaluator", To: "sub_flow", Route: "passed"},
		},
	}

	subs := cfg.GetSubAgents()
	if len(subs) != 4 {
		t.Fatalf("expected 4 sub agents, got %d: %v", len(subs), subs)
	}

	mermaid := cfg.ExportMermaid("TestWorkflow")
	if !strings.Contains(mermaid, "START --> drafter") {
		t.Errorf("mermaid output missing START -> drafter: %s", mermaid)
	}
	if !strings.Contains(mermaid, "evaluator -->|needs_revision| refinement") {
		t.Errorf("mermaid output missing cyclic route edge: %s", mermaid)
	}
	if !strings.Contains(mermaid, "evaluator -->|passed| sub_flow") {
		t.Errorf("mermaid output missing passed route edge: %s", mermaid)
	}
}

func TestParseRoute(t *testing.T) {
	rDef := parseRoute("DEFAULT")
	if rDef == nil {
		t.Error("expected default route, got nil")
	}

	rBool := parseRoute("true")
	if rBool == nil {
		t.Error("expected bool route, got nil")
	}

	rInt := parseRoute("42")
	if rInt == nil {
		t.Error("expected int route, got nil")
	}

	rStr := parseRoute("custom_route")
	if rStr == nil {
		t.Error("expected string route, got nil")
	}
}
