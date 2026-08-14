package main

import (
	"encoding/json"
	"testing"
)

func TestStripRouteBoundStateOnlyTouchesTopLevelState(t *testing.T) {
	payload := []byte(`{
  "model":"gpt-5.6-terra",
  "previous_response_id":"resp_old",
  "input":[
    {"type":"reasoning","id":"rs_top","encrypted_content":"top-secret"},
    {"type":"compaction","encrypted_content":"compact-secret"},
    {"type":"agent_message","author":"/root/child","recipient":"/root","content":[{"type":"encrypted_content","encrypted_content":"child-secret"}],"internal_chat_message_metadata_passthrough":{"turn_id":"child-turn"}},
    {"type":"function_call_output","call_id":"call_1","output":"done"}
  ],
  "tools":[{"type":"function","name":"spawn_agent"}]
}`)

	inspection, errInspect := inspectPayload(payload)
	if errInspect != nil {
		t.Fatalf("inspectPayload() error = %v", errInspect)
	}
	if !inspection.Signals.HasReasoning || !inspection.Signals.HasCompaction || !inspection.Signals.HasPreviousResponseID {
		t.Fatalf("signals = %#v", inspection.Signals)
	}

	updated, changed, errStrip := inspection.stripRouteBoundState()
	if errStrip != nil {
		t.Fatalf("stripRouteBoundState() error = %v", errStrip)
	}
	if !changed {
		t.Fatal("stripRouteBoundState() changed = false, want true")
	}

	var root map[string]json.RawMessage
	if errUnmarshal := json.Unmarshal(updated, &root); errUnmarshal != nil {
		t.Fatalf("decode updated payload: %v", errUnmarshal)
	}
	if _, exists := root["previous_response_id"]; exists {
		t.Fatalf("previous_response_id remained: %s", updated)
	}
	var input []json.RawMessage
	if errUnmarshal := json.Unmarshal(root["input"], &input); errUnmarshal != nil {
		t.Fatalf("decode input: %v", errUnmarshal)
	}
	if len(input) != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", len(input), updated)
	}
	var agent map[string]any
	if errUnmarshal := json.Unmarshal(input[0], &agent); errUnmarshal != nil {
		t.Fatalf("decode agent message: %v", errUnmarshal)
	}
	if got, _ := agent["type"].(string); got != "agent_message" {
		t.Fatalf("input[0].type = %q, want agent_message", got)
	}
	content, ok := agent["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("agent content = %#v", agent["content"])
	}
	part, ok := content[0].(map[string]any)
	if !ok || part["type"] != "encrypted_content" || part["encrypted_content"] != "child-secret" {
		t.Fatalf("nested encrypted agent content was changed: %#v", content[0])
	}
	if got, _ := agent["author"].(string); got != "/root/child" {
		t.Fatalf("agent author = %q", got)
	}
	if got, _ := agent["recipient"].(string); got != "/root" {
		t.Fatalf("agent recipient = %q", got)
	}
	if got := string(input[1]); got == "" || !containsJSONType(input[1], "function_call_output") {
		t.Fatalf("function output changed: %s", got)
	}
	if _, exists := root["tools"]; !exists {
		t.Fatalf("tools were removed: %s", updated)
	}
}

func TestStripRouteBoundStateKeepsCleanPayloadByteForByte(t *testing.T) {
	payload := []byte(`{"model":"gpt-5.6-terra","input":[{"type":"message","role":"user","content":"hello"}]}`)
	inspection, errInspect := inspectPayload(payload)
	if errInspect != nil {
		t.Fatalf("inspectPayload() error = %v", errInspect)
	}
	updated, changed, errStrip := inspection.stripRouteBoundState()
	if errStrip != nil {
		t.Fatalf("stripRouteBoundState() error = %v", errStrip)
	}
	if changed {
		t.Fatal("stripRouteBoundState() changed = true, want false")
	}
	if string(updated) != string(payload) {
		t.Fatalf("clean payload changed\n got: %s\nwant: %s", updated, payload)
	}
}

func TestInspectPayloadAcceptsNonArrayInput(t *testing.T) {
	inspection, errInspect := inspectPayload([]byte(`{"input":"hello","previous_response_id":"resp_old"}`))
	if errInspect != nil {
		t.Fatalf("inspectPayload() error = %v", errInspect)
	}
	if inspection.InputArray {
		t.Fatal("InputArray = true, want false")
	}
	if !inspection.Signals.HasPreviousResponseID {
		t.Fatalf("signals = %#v", inspection.Signals)
	}
}

func containsJSONType(raw json.RawMessage, want string) bool {
	var value struct {
		Type string `json:"type"`
	}
	return json.Unmarshal(raw, &value) == nil && value.Type == want
}
