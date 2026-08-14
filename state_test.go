package main

import (
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestRouteStatePreservesSameSuccessfulRouteAndStripsChangedRoute(t *testing.T) {
	store := newRouteStateStore(testConfig())
	session := "session-a"
	routeA := routeFingerprint{Digest: "route-a"}
	routeB := routeFingerprint{Digest: "route-b"}

	first := store.prepare("request-1", session, routeA, true, stateSignals{})
	if first.Strip || first.Reject {
		t.Fatalf("first decision = %#v", first)
	}
	store.complete(pluginapi.RequestCompletion{RequestID: "request-1", Outcome: pluginapi.RequestCompletionSucceeded})

	same := store.prepare("request-2", session, routeA, true, stateSignals{HasReasoning: true, HasPreviousResponseID: true})
	if same.Relation != routeRelationSame || same.Strip || same.Reject {
		t.Fatalf("same-route decision = %#v", same)
	}

	changed := store.prepare("request-3", session, routeB, true, stateSignals{HasReasoning: true, HasPreviousResponseID: true})
	if changed.Relation != routeRelationChanged || !changed.Strip || changed.Reject {
		t.Fatalf("changed-route decision = %#v", changed)
	}
}

func TestRouteStateDoesNotCommitFailedCandidate(t *testing.T) {
	store := newRouteStateStore(testConfig())
	session := "session-a"
	routeA := routeFingerprint{Digest: "route-a"}
	routeB := routeFingerprint{Digest: "route-b"}

	store.prepare("initial", session, routeA, true, stateSignals{})
	store.complete(pluginapi.RequestCompletion{RequestID: "initial", Outcome: pluginapi.RequestCompletionSucceeded})

	candidate := store.prepare("failed-candidate", session, routeB, true, stateSignals{HasReasoning: true})
	if !candidate.Strip {
		t.Fatalf("candidate decision = %#v, want strip", candidate)
	}
	store.complete(pluginapi.RequestCompletion{RequestID: "failed-candidate", Outcome: pluginapi.RequestCompletionFailed, Error: "upstream unavailable"})

	stillA := store.prepare("same-a", session, routeA, true, stateSignals{HasReasoning: true})
	if stillA.Relation != routeRelationSame || stillA.Strip {
		t.Fatalf("state after failed candidate = %#v", stillA)
	}
	newB := store.prepare("retry-b", session, routeB, true, stateSignals{HasReasoning: true})
	if newB.Relation != routeRelationChanged || !newB.Strip {
		t.Fatalf("B after failed candidate = %#v", newB)
	}
}

func TestRouteStateUsesLastSelectedAttemptForRetry(t *testing.T) {
	store := newRouteStateStore(testConfig())
	session := "session-a"
	routeA := routeFingerprint{Digest: "route-a"}
	routeB := routeFingerprint{Digest: "route-b"}

	store.prepare("retrying-request", session, routeA, true, stateSignals{})
	store.prepare("retrying-request", session, routeB, true, stateSignals{})
	store.complete(pluginapi.RequestCompletion{RequestID: "retrying-request", Outcome: pluginapi.RequestCompletionSucceeded})

	decision := store.prepare("next", session, routeB, true, stateSignals{HasReasoning: true})
	if decision.Relation != routeRelationSame || decision.Strip || decision.Reject {
		t.Fatalf("retry commit decision = %#v", decision)
	}
}

func TestRouteStateConcurrentRouteChangesStayTaintedAndCannotRollBackNewerRoute(t *testing.T) {
	store := newRouteStateStore(testConfig())
	session := "session-a"
	routeA := routeFingerprint{Digest: "route-a"}
	routeB := routeFingerprint{Digest: "route-b"}
	routeC := routeFingerprint{Digest: "route-c"}

	store.prepare("initial", session, routeA, true, stateSignals{})
	store.complete(pluginapi.RequestCompletion{RequestID: "initial", Outcome: pluginapi.RequestCompletionSucceeded})
	store.prepare("older", session, routeB, true, stateSignals{HasReasoning: true})
	store.prepare("newer", session, routeC, true, stateSignals{HasReasoning: true})

	store.complete(pluginapi.RequestCompletion{RequestID: "newer", Outcome: pluginapi.RequestCompletionSucceeded})
	store.complete(pluginapi.RequestCompletion{RequestID: "older", Outcome: pluginapi.RequestCompletionSucceeded})

	state := store.sessions["session-a"]
	if !state.Route.equal(routeC) || !state.Tainted {
		t.Fatalf("state after concurrent route changes = %#v", state)
	}
	decision := store.prepare("verify", session, routeC, true, stateSignals{HasReasoning: true})
	if decision.Relation != routeRelationChanged || !decision.Strip || decision.Reject {
		t.Fatalf("tainted route state = %#v", decision)
	}
}

func TestRouteStateSameRouteConcurrencyDoesNotTaint(t *testing.T) {
	store := newRouteStateStore(testConfig())
	route := routeFingerprint{Digest: "route-a"}
	seedCommittedRoute(store, "session-a", route)

	first := store.prepare("first", "session-a", route, true, stateSignals{HasReasoning: true})
	second := store.prepare("second", "session-a", route, true, stateSignals{HasReasoning: true})
	if first.Strip || second.Strip || first.Reject || second.Reject {
		t.Fatalf("same-route concurrent decisions = %#v, %#v", first, second)
	}
	if state := store.sessions["session-a"]; state.Tainted {
		t.Fatalf("same-route concurrency tainted state: %#v", state)
	}
}

func TestRouteStateTaintClearsOnlyAfterCleanSuccessfulTurn(t *testing.T) {
	store := newRouteStateStore(testConfig())
	routeA := routeFingerprint{Digest: "route-a"}
	routeB := routeFingerprint{Digest: "route-b"}
	seedCommittedRoute(store, "session-a", routeA)

	store.prepare("older", "session-a", routeA, true, stateSignals{HasReasoning: true})
	store.prepare("newer", "session-a", routeB, true, stateSignals{HasReasoning: true})
	store.complete(pluginapi.RequestCompletion{RequestID: "newer", Outcome: pluginapi.RequestCompletionSucceeded})
	store.complete(pluginapi.RequestCompletion{RequestID: "older", Outcome: pluginapi.RequestCompletionSucceeded})
	if !store.sessions["session-a"].Tainted {
		t.Fatal("expected concurrent cross-route state to be tainted")
	}

	// A clean request can establish a new unambiguous route only after the
	// conflicting requests have finished.
	clean := store.prepare("clean", "session-a", routeB, true, stateSignals{})
	if clean.Strip || clean.Reject {
		t.Fatalf("clean reset decision = %#v", clean)
	}
	store.complete(pluginapi.RequestCompletion{RequestID: "clean", Outcome: pluginapi.RequestCompletionSucceeded})
	if state := store.sessions["session-a"]; state.Tainted || !state.Route.equal(routeB) {
		t.Fatalf("clean reset did not establish route B: %#v", state)
	}

	resumed := store.prepare("resumed", "session-a", routeB, true, stateSignals{HasReasoning: true})
	if resumed.Relation != routeRelationSame || resumed.Strip || resumed.Reject {
		t.Fatalf("resumed reasoning decision = %#v", resumed)
	}
}

func TestRouteStateCompactionPolicies(t *testing.T) {
	blockCfg := testConfig()
	blockStore := newRouteStateStore(blockCfg)
	seedCommittedRoute(blockStore, "session-a", routeFingerprint{Digest: "route-a"})
	blocked := blockStore.prepare("block", "session-a", routeFingerprint{Digest: "route-b"}, true, stateSignals{HasCompaction: true})
	if !blocked.Reject || blocked.Strip {
		t.Fatalf("block decision = %#v", blocked)
	}
	if len(blockStore.pending) != 0 {
		t.Fatalf("blocked request became pending: %#v", blockStore.pending)
	}

	stripCfg := testConfig()
	stripCfg.CompactionPolicy = compactionPolicyStrip
	stripStore := newRouteStateStore(stripCfg)
	seedCommittedRoute(stripStore, "session-a", routeFingerprint{Digest: "route-a"})
	stripped := stripStore.prepare("strip", "session-a", routeFingerprint{Digest: "route-b"}, true, stateSignals{HasCompaction: true})
	if stripped.Reject || !stripped.Strip {
		t.Fatalf("strip decision = %#v", stripped)
	}
}

func TestRouteStateExpiresAndBoundsEntries(t *testing.T) {
	now := time.Date(2026, time.August, 13, 12, 0, 0, 0, time.UTC)
	cfg := testConfig()
	cfg.StateTTL = time.Minute
	cfg.MaxSessions = 1
	cfg.MaxPending = 1
	store := newRouteStateStoreWithClock(cfg, func() time.Time { return now })
	seedCommittedRoute(store, "session-a", routeFingerprint{Digest: "route-a"})
	now = now.Add(2 * time.Minute)
	expired := store.prepare("after-ttl", "session-a", routeFingerprint{Digest: "route-a"}, true, stateSignals{HasReasoning: true})
	if expired.Relation != routeRelationUnknown || !expired.Strip {
		t.Fatalf("expired decision = %#v", expired)
	}

	store.prepare("pending-a", "session-a", routeFingerprint{Digest: "route-a"}, true, stateSignals{})
	now = now.Add(time.Second)
	store.prepare("pending-b", "session-b", routeFingerprint{Digest: "route-b"}, true, stateSignals{})
	if len(store.pending) != 1 {
		t.Fatalf("pending length = %d, want 1", len(store.pending))
	}
	store.complete(pluginapi.RequestCompletion{RequestID: "pending-b", Outcome: pluginapi.RequestCompletionSucceeded})
	if len(store.sessions) != 1 {
		t.Fatalf("session length = %d, want 1", len(store.sessions))
	}
}

func TestInvalidEncryptedContentInvalidatesMatchingCommittedRoute(t *testing.T) {
	store := newRouteStateStore(testConfig())
	route := routeFingerprint{Digest: "route-a"}
	seedCommittedRoute(store, "session-a", route)
	store.prepare("bad", "session-a", route, true, stateSignals{HasReasoning: true})
	store.complete(pluginapi.RequestCompletion{
		RequestID: "bad",
		Outcome:   pluginapi.RequestCompletionFailed,
		Error:     "thinking_signature_invalid: invalid_encrypted_content",
	})
	decision := store.prepare("next", "session-a", route, true, stateSignals{HasReasoning: true})
	if decision.Relation != routeRelationUnknown || !decision.Strip {
		t.Fatalf("invalidated route decision = %#v", decision)
	}
}

func testConfig() pluginConfig {
	cfg := defaultPluginConfig()
	cfg.StateTTL = time.Hour
	cfg.MaxSessions = 16
	cfg.MaxPending = 16
	return cfg
}

func seedCommittedRoute(store *routeStateStore, session string, route routeFingerprint) {
	store.prepare("seed-"+session, session, route, true, stateSignals{})
	store.complete(pluginapi.RequestCompletion{RequestID: "seed-" + session, Outcome: pluginapi.RequestCompletionSucceeded})
}
