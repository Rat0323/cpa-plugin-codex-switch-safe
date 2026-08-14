package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type stateSignals struct {
	HasReasoning          bool
	HasCompaction         bool
	HasPreviousResponseID bool
}

func (s stateSignals) hasRouteBoundState() bool {
	return s.HasReasoning || s.HasCompaction || s.HasPreviousResponseID
}

type payloadInspection struct {
	Raw        []byte
	Root       map[string]json.RawMessage
	InputItems []json.RawMessage
	InputArray bool
	Signals    stateSignals
}

func inspectPayload(raw []byte) (payloadInspection, error) {
	inspection := payloadInspection{Raw: raw}
	if len(bytes.TrimSpace(raw)) == 0 {
		return inspection, fmt.Errorf("request body is empty")
	}
	if errUnmarshal := json.Unmarshal(raw, &inspection.Root); errUnmarshal != nil {
		return inspection, fmt.Errorf("decode JSON body: %w", errUnmarshal)
	}
	if inspection.Root == nil {
		return inspection, fmt.Errorf("request body must be a JSON object")
	}
	if _, exists := inspection.Root["previous_response_id"]; exists {
		inspection.Signals.HasPreviousResponseID = true
	}

	input, exists := inspection.Root["input"]
	if !exists || len(bytes.TrimSpace(input)) == 0 || bytes.Equal(bytes.TrimSpace(input), []byte("null")) {
		return inspection, nil
	}
	if errUnmarshal := json.Unmarshal(input, &inspection.InputItems); errUnmarshal != nil {
		return inspection, nil
	}
	inspection.InputArray = true
	for _, item := range inspection.InputItems {
		var kind struct {
			Type string `json:"type"`
		}
		if errUnmarshal := json.Unmarshal(item, &kind); errUnmarshal != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(kind.Type)) {
		case "reasoning":
			inspection.Signals.HasReasoning = true
		case "compaction":
			inspection.Signals.HasCompaction = true
		}
	}
	return inspection, nil
}

// stripRouteBoundState deliberately changes only top-level Responses input
// items. It never recursively searches for encrypted_content, because nested
// agent messages and tool payloads are independent application data.
func (p payloadInspection) stripRouteBoundState() ([]byte, bool, error) {
	if !p.Signals.hasRouteBoundState() {
		return p.Raw, false, nil
	}
	changed := false
	if _, exists := p.Root["previous_response_id"]; exists {
		delete(p.Root, "previous_response_id")
		changed = true
	}
	if p.InputArray {
		filtered := make([]json.RawMessage, 0, len(p.InputItems))
		for _, item := range p.InputItems {
			var kind struct {
				Type string `json:"type"`
			}
			if errUnmarshal := json.Unmarshal(item, &kind); errUnmarshal != nil {
				filtered = append(filtered, item)
				continue
			}
			switch strings.ToLower(strings.TrimSpace(kind.Type)) {
			case "reasoning", "compaction":
				changed = true
				continue
			default:
				filtered = append(filtered, item)
			}
		}
		if changed {
			rawInput, errMarshal := json.Marshal(filtered)
			if errMarshal != nil {
				return nil, false, fmt.Errorf("encode filtered input: %w", errMarshal)
			}
			p.Root["input"] = rawInput
		}
	}
	if !changed {
		return p.Raw, false, nil
	}
	updated, errMarshal := json.Marshal(p.Root)
	if errMarshal != nil {
		return nil, false, fmt.Errorf("encode sanitized request: %w", errMarshal)
	}
	return updated, true, nil
}

func likelyContainsRouteBoundState(raw []byte) bool {
	lower := bytes.ToLower(raw)
	return bytes.Contains(lower, []byte("\"previous_response_id\"")) ||
		bytes.Contains(lower, []byte("\"reasoning\"")) ||
		bytes.Contains(lower, []byte("\"compaction\""))
}
