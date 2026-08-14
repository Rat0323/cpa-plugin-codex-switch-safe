package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

type switchSafePlugin struct {
	cfg   pluginConfig
	state *routeStateStore
}

var _ pluginapi.RequestInterceptor = (*switchSafePlugin)(nil)
var _ pluginapi.RequestLifecyclePlugin = (*switchSafePlugin)(nil)

func (p *switchSafePlugin) InterceptRequestBeforeAuth(context.Context, pluginapi.RequestInterceptRequest) (pluginapi.RequestInterceptResponse, error) {
	// The selected auth is unavailable before this hook, so changing state here
	// would make retry/failover behavior unsafe.
	return pluginapi.RequestInterceptResponse{}, nil
}

func (p *switchSafePlugin) InterceptRequestAfterAuth(_ context.Context, req pluginapi.RequestInterceptRequest) (pluginapi.RequestInterceptResponse, error) {
	if p == nil || !p.cfg.Enabled || !isCodexTarget(req) {
		return pluginapi.RequestInterceptResponse{}, nil
	}

	inspection, errInspect := inspectPayload(req.Body)
	if errInspect != nil {
		if likelyContainsRouteBoundState(req.Body) {
			return stateResetTermination(http.StatusBadRequest, "Codex route-bound state could not be parsed safely; start a fresh turn."), nil
		}
		return pluginapi.RequestInterceptResponse{}, nil
	}

	sessionKey := sessionKeyFromRequest(req, inspection)
	route, routeKnown := routeFingerprintFromRequest(req)
	decision := p.state.prepare(req.RequestID, sessionKey, route, routeKnown, inspection.Signals)
	if decision.Reject {
		return compactionTermination(), nil
	}
	if !decision.Strip {
		return pluginapi.RequestInterceptResponse{}, nil
	}

	updated, changed, errStrip := inspection.stripRouteBoundState()
	if errStrip != nil || !changed {
		p.state.discardPending(req.RequestID)
		return stateResetTermination(http.StatusConflict, "Codex route-bound state could not be reset safely; start a fresh turn."), nil
	}
	return pluginapi.RequestInterceptResponse{Body: updated}, nil
}

func (p *switchSafePlugin) HandleRequestComplete(_ context.Context, completion pluginapi.RequestCompletion) error {
	if p == nil || !p.cfg.Enabled {
		return nil
	}
	p.state.complete(completion)
	return nil
}

func isCodexTarget(req pluginapi.RequestInterceptRequest) bool {
	return strings.EqualFold(strings.TrimSpace(req.ToFormat), "codex")
}

func compactionTermination() pluginapi.RequestInterceptResponse {
	return stateResetTermination(http.StatusConflict, "Codex compaction belongs to a different or unknown upstream credential. Retry on the original credential, or set compaction_policy: strip to continue without compaction.")
}

func stateResetTermination(status int, message string) pluginapi.RequestInterceptResponse {
	body, errMarshal := json.Marshal(map[string]any{
		"error": map[string]any{
			"type":    "codex_switch_safe_reset_required",
			"message": message,
		},
	})
	if errMarshal != nil {
		body = []byte(`{"error":{"type":"codex_switch_safe_reset_required","message":"Codex route-bound state must be reset."}}`)
	}
	return pluginapi.RequestInterceptResponse{
		Terminate:       true,
		StatusCode:      status,
		ResponseHeaders: http.Header{"Content-Type": []string{"application/json"}},
		ResponseBody:    body,
	}
}
