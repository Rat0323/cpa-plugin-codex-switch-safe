package main

import (
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestSessionKeyUsesStableCodexConversationNotTurnID(t *testing.T) {
	first := testSessionRequest("session-a", "thread-a", "turn-1", "caller-a")
	second := testSessionRequest("session-a", "thread-a", "turn-2", "caller-a")

	firstInspection, errFirst := inspectPayload(first.Body)
	if errFirst != nil {
		t.Fatal(errFirst)
	}
	secondInspection, errSecond := inspectPayload(second.Body)
	if errSecond != nil {
		t.Fatal(errSecond)
	}
	firstKey := sessionKeyFromRequest(first, firstInspection)
	secondKey := sessionKeyFromRequest(second, secondInspection)
	if firstKey == "" || secondKey == "" {
		t.Fatalf("session keys = %q, %q", firstKey, secondKey)
	}
	if firstKey != secondKey {
		t.Fatalf("turn changed session key\nfirst=%s\nsecond=%s", firstKey, secondKey)
	}
}

func TestSessionKeySeparatesChildConversationAndCaller(t *testing.T) {
	parent := testSessionRequest("session-parent", "thread-parent", "turn-parent", "caller-a")
	child := testSessionRequest("session-child", "thread-child", "turn-child", "caller-a")
	otherCaller := testSessionRequest("session-parent", "thread-parent", "turn-other", "caller-b")

	parentInspection, _ := inspectPayload(parent.Body)
	childInspection, _ := inspectPayload(child.Body)
	otherInspection, _ := inspectPayload(otherCaller.Body)
	parentKey := sessionKeyFromRequest(parent, parentInspection)
	childKey := sessionKeyFromRequest(child, childInspection)
	otherCallerKey := sessionKeyFromRequest(otherCaller, otherInspection)
	if parentKey == childKey {
		t.Fatalf("child conversation shared parent state key: %s", parentKey)
	}
	if parentKey == otherCallerKey {
		t.Fatalf("caller scope shared state key: %s", parentKey)
	}
}

func TestSessionKeyPrefersExecutionSessionForNestedAgents(t *testing.T) {
	parent := testSessionRequest("shared-session", "shared-thread", "turn-parent", "caller-a")
	child := testSessionRequest("shared-session", "shared-thread", "turn-child", "caller-a")
	parent.Metadata["execution_session_id"] = "execution-parent"
	child.Metadata["execution_session_id"] = "execution-child"

	parentInspection, errParent := inspectPayload(parent.Body)
	if errParent != nil {
		t.Fatal(errParent)
	}
	childInspection, errChild := inspectPayload(child.Body)
	if errChild != nil {
		t.Fatal(errChild)
	}
	parentKey := sessionKeyFromRequest(parent, parentInspection)
	childKey := sessionKeyFromRequest(child, childInspection)
	if parentKey == "" || childKey == "" || parentKey == childKey {
		t.Fatalf("execution session keys = %q, %q", parentKey, childKey)
	}
}

func TestSessionKeyNeverUsesAgentMessageContents(t *testing.T) {
	request := testSessionRequest("session-root", "thread-root", "turn-root", "caller-a")
	request.Body = []byte(`{"input":[{"type":"agent_message","author":"/root","recipient":"/root/worker","content":[{"type":"encrypted_content","encrypted_content":"child-private-state"}]}]}`)
	inspection, errInspect := inspectPayload(request.Body)
	if errInspect != nil {
		t.Fatal(errInspect)
	}
	key := sessionKeyFromRequest(request, inspection)
	if key == "" {
		t.Fatal("session key = empty")
	}
	if key == opaqueDigest("session-v1", "caller-a", "agent_message", "child-private-state") {
		t.Fatal("session key was derived from nested agent message content")
	}
}

func TestSessionKeyWithoutStableIdentityIsEmpty(t *testing.T) {
	req := pluginapi.RequestInterceptRequest{Body: []byte(`{"input":[{"type":"reasoning","encrypted_content":"opaque"}]}`)}
	inspection, errInspect := inspectPayload(req.Body)
	if errInspect != nil {
		t.Fatal(errInspect)
	}
	if key := sessionKeyFromRequest(req, inspection); key != "" {
		t.Fatalf("session key = %q, want empty", key)
	}
}

func TestSessionKeyDoesNotTreatPromptCacheOrWindowAsConversation(t *testing.T) {
	req := pluginapi.RequestInterceptRequest{
		Body: []byte(`{"prompt_cache_key":"cache-a","window_id":"window-a","input":[{"type":"reasoning","encrypted_content":"opaque"}]}`),
	}
	inspection, errInspect := inspectPayload(req.Body)
	if errInspect != nil {
		t.Fatal(errInspect)
	}
	if key := sessionKeyFromRequest(req, inspection); key != "" {
		t.Fatalf("session key = %q, want empty", key)
	}
}

func testSessionRequest(sessionID, threadID, turnID, callerScope string) pluginapi.RequestInterceptRequest {
	return pluginapi.RequestInterceptRequest{
		Headers: http.Header{
			"Session-Id":            []string{sessionID},
			"Thread-Id":             []string{threadID},
			"X-Codex-Turn-Metadata": []string{`{"session_id":"` + sessionID + `","thread_id":"` + threadID + `","turn_id":"` + turnID + `"}`},
		},
		Body:     []byte(`{"input":[{"type":"message","role":"user","content":"hello"}]}`),
		Metadata: map[string]any{"caller_scope": callerScope},
	}
}
