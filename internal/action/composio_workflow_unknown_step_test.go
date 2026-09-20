package action

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// savedWorkflowWithUnknownStep is the shape of a workflow saved by a build
// whose step-type set differed (a retired type, or one from a newer build).
// Definitions like this are on real users' disks, so both halves of the
// behaviour below are load-bearing.
const savedWorkflowWithUnknownStep = `{
  "version": "hivex_workflow_v1",
  "title": "Daily digest",
  "steps": [
    {"id": "summary", "type": "template", "template": "hello"},
    {"id": "compose", "type": "legacy_ask", "query_template": "Draft a digest from {{ .steps.summary.result }}"}
  ]}`

// TestUnknownWorkflowStepStillDecodes: a saved workflow containing an unknown
// step type must still LOAD. Rejecting it at decode time would make the
// workflow unlistable and uneditable, hiding it from the person who has to fix
// it — a worse outcome than a workflow that loads and refuses to run one step.
func TestUnknownWorkflowStepStillDecodes(t *testing.T) {
	c := &ComposioREST{}
	spec, err := c.decodeWorkflowDefinition(json.RawMessage(savedWorkflowWithUnknownStep))
	if err != nil {
		t.Fatalf("saved workflow with an unknown step must still decode, got: %v", err)
	}
	if len(spec.Steps) != 2 {
		t.Fatalf("expected both steps preserved, got %d", len(spec.Steps))
	}
	if spec.Steps[1].Type != "legacy_ask" {
		t.Fatalf("unknown step type should round-trip unchanged, got %q", spec.Steps[1].Type)
	}
	if strings.TrimSpace(spec.Steps[1].QueryTemplate) == "" {
		t.Fatal("unknown step's query_template must survive the round trip so the operator can still see it")
	}
}

// TestUnknownWorkflowStepFailsLoudlyOnExecute: executing an unknown step must
// produce a named, actionable error — never a silent skip and never a
// substituted result, either of which would look like the workflow worked.
func TestUnknownWorkflowStepFailsLoudlyOnExecute(t *testing.T) {
	c := &ComposioREST{}
	for _, stepType := range []string{"legacy_ask", "legacy_insights"} {
		t.Run(stepType, func(t *testing.T) {
			step := workflowStep{ID: "compose", Type: stepType, QueryTemplate: "anything"}
			out, err := c.executeWorkflowStep(context.Background(), step, map[string]any{}, false)
			if err == nil {
				t.Fatalf("expected an error for unknown step type %q, got output %v", stepType, out)
			}
			if out != nil {
				t.Fatalf("a refused step must return no result, got %v", out)
			}
			msg := err.Error()
			if !strings.Contains(msg, "compose") || !strings.Contains(msg, stepType) {
				t.Fatalf("error must name the step and the step type so the operator can find it, got: %s", msg)
			}
		})
	}
}
