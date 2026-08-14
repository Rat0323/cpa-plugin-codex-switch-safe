package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestInterceptorPreservesSameRouteAndStripsChangedRoute(t *testing.T) {
	p, errBuild := buildPlugin([]byte("enabled: true\n"))
	if errBuild != nil {
		t.Fatal(errBuild)
	}
	first := codexRequest("req-1", "session-a", "thread-a", "auth-a", "gpt-5.6-a", `{"input":[{"type":"message","role":"user","content":"hi"}]}`)
	if resp, errIntercept := p.InterceptRequestAfterAuth(nil, first); errIntercept != nil || resp.Terminate || len(resp.Body) != 0 {
		t.Fatalf("first response = %#v, err=%v", resp, errIntercept)
	}
	p.HandleRequestComplete(nil, pluginapi.RequestCompletion{RequestID: "req-1", Outcome: pluginapi.RequestCompletionSucceeded})

	same := codexRequest("req-2", "session-a", "thread-a", "auth-a", "gpt-5.6-a", `{"previous_response_id":"resp-a","input":[{"type":"reasoning","encrypted_content":"same"},{"type":"message","role":"user","content":"continue"}]}`)
	if resp, errIntercept := p.InterceptRequestAfterAuth(nil, same); errIntercept != nil || resp.Terminate || len(resp.Body) != 0 {
		t.Fatalf("same response = %#v, err=%v", resp, errIntercept)
	}
	p.HandleRequestComplete(nil, pluginapi.RequestCompletion{RequestID: "req-2", Outcome: pluginapi.RequestCompletionSucceeded})

	changed := codexRequest("req-3", "session-a", "thread-a", "auth-b", "gpt-5.6-b", `{"previous_response_id":"resp-a","input":[{"type":"reasoning","encrypted_content":"foreign"},{"type":"agent_message","author":"/root","recipient":"/root/child","content":[{"type":"encrypted_content","encrypted_content":"child"}]}]}`)
	resp, errIntercept := p.InterceptRequestAfterAuth(nil, changed)
	if errIntercept != nil || resp.Terminate || len(resp.Body) == 0 {
		t.Fatalf("changed response = %#v, err=%v", resp, errIntercept)
	}
	var body map[string]any
	if errDecode := json.Unmarshal(resp.Body, &body); errDecode != nil {
		t.Fatal(errDecode)
	}
	if _, exists := body["previous_response_id"]; exists {
		t.Fatalf("previous_response_id survived: %s", resp.Body)
	}
	if strings.Contains(string(resp.Body), `"type":"reasoning"`) {
		t.Fatalf("reasoning survived: %s", resp.Body)
	}
	if !strings.Contains(string(resp.Body), "child") {
		t.Fatalf("nested agent message was removed: %s", resp.Body)
	}
}

func TestInterceptorBlocksUnsafeCompactionByDefault(t *testing.T) {
	p, errBuild := buildPlugin([]byte("enabled: true\ncompaction_policy: block\n"))
	if errBuild != nil {
		t.Fatal(errBuild)
	}
	seed := codexRequest("seed", "session-a", "thread-a", "auth-a", "gpt-5.6-a", `{"input":[{"type":"message","role":"user","content":"hi"}]}`)
	p.InterceptRequestAfterAuth(nil, seed)
	p.HandleRequestComplete(nil, pluginapi.RequestCompletion{RequestID: "seed", Outcome: pluginapi.RequestCompletionSucceeded})
	request := codexRequest("compact", "session-a", "thread-a", "auth-b", "gpt-5.6-b", `{"input":[{"type":"compaction","encrypted_content":"foreign"}]}`)
	resp, errIntercept := p.InterceptRequestAfterAuth(nil, request)
	if errIntercept != nil || !resp.Terminate || resp.StatusCode != http.StatusConflict {
		t.Fatalf("compaction response = %#v, err=%v", resp, errIntercept)
	}
}

func TestInterceptorFailClosedWithoutSelectedAuthOrSession(t *testing.T) {
	p, errBuild := buildPlugin(nil)
	if errBuild != nil {
		t.Fatal(errBuild)
	}
	request := pluginapi.RequestInterceptRequest{
		RequestID: "unknown",
		ToFormat:  "codex",
		Model:     "gpt-5.6",
		Body:      []byte(`{"previous_response_id":"resp","input":[{"type":"reasoning","encrypted_content":"opaque"}]}`),
	}
	resp, errIntercept := p.InterceptRequestAfterAuth(nil, request)
	if errIntercept != nil || resp.Terminate || len(resp.Body) == 0 || strings.Contains(string(resp.Body), "encrypted_content") {
		t.Fatalf("fail-closed response = %#v, err=%v", resp, errIntercept)
	}
}

func codexRequest(requestID, sessionID, threadID, authID, model, body string) pluginapi.RequestInterceptRequest {
	return pluginapi.RequestInterceptRequest{
		RequestID: requestID,
		ToFormat:  "codex",
		Model:     model,
		Headers: http.Header{
			"Session-Id":            []string{sessionID},
			"Thread-Id":             []string{threadID},
			"X-Codex-Turn-Metadata": []string{`{"session_id":"` + sessionID + `","thread_id":"` + threadID + `"}`},
		},
		Metadata: map[string]any{"caller_scope": "desktop", "selected_auth_id": authID},
		Body:     []byte(body),
	}
}
