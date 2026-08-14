package main

import (
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

type routeFingerprint struct {
	Digest string
}

// opaqueDigestKey is process-local on purpose. The plugin never persists
// routing state, and a randomized key avoids making in-memory identifiers
// linkable across restarts or plugin reloads.
var opaqueDigestKey = newOpaqueDigestKey()

func newOpaqueDigestKey() []byte {
	key := make([]byte, sha256.Size)
	if _, errRead := cryptorand.Read(key); errRead == nil {
		return key
	}

	// crypto/rand is expected to be available on supported CPA hosts. Keep a
	// per-process fallback so an unexpected OS entropy failure does not turn
	// the digest into a stable cross-restart identifier.
	fallback := sha256.Sum256([]byte(pluginID + "\x00" + strconv.FormatInt(time.Now().UnixNano(), 10)))
	return fallback[:]
}

func routeFingerprintFromRequest(req pluginapi.RequestInterceptRequest) (routeFingerprint, bool) {
	authID := normalizeOpaqueIdentifier(metadataString(req.Metadata, "selected_auth_id"))
	model := normalizeOpaqueIdentifier(req.Model)
	toFormat := strings.ToLower(strings.TrimSpace(req.ToFormat))
	if authID == "" || model == "" || toFormat != "codex" {
		return routeFingerprint{}, false
	}
	return routeFingerprint{Digest: opaqueDigest("route-v1", authID, model, toFormat)}, true
}

func (r routeFingerprint) equal(other routeFingerprint) bool {
	return r.Digest != "" && r.Digest == other.Digest
}

func opaqueDigest(namespace string, parts ...string) string {
	hash := hmac.New(sha256.New, opaqueDigestKey)
	_, _ = hash.Write([]byte(pluginID))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(namespace))
	for _, part := range parts {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(part))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type routeRelation uint8

const (
	routeRelationUnknown routeRelation = iota
	routeRelationSame
	routeRelationChanged
)

type routeDecision struct {
	Relation routeRelation
	Strip    bool
	Reject   bool
}

type sessionRouteState struct {
	Route     routeFingerprint
	HasRoute  bool
	Sequence  uint64
	UpdatedAt time.Time
	Tainted   bool
}

type pendingRoute struct {
	SessionKey    string
	Route         routeFingerprint
	Sequence      uint64
	ObservedAt    time.Time
	CleanStart    bool
	CanClearTaint bool
}

type routeStateStore struct {
	mu       sync.Mutex
	cfg      pluginConfig
	now      func() time.Time
	sequence uint64
	sessions map[string]sessionRouteState
	pending  map[string]pendingRoute
}

func newRouteStateStore(cfg pluginConfig) *routeStateStore {
	return newRouteStateStoreWithClock(cfg, time.Now)
}

func newRouteStateStoreWithClock(cfg pluginConfig, now func() time.Time) *routeStateStore {
	if now == nil {
		now = time.Now
	}
	return &routeStateStore{
		cfg:      cfg,
		now:      now,
		sessions: make(map[string]sessionRouteState),
		pending:  make(map[string]pendingRoute),
	}
}

// prepare records only the final selected-auth candidate for a logical request.
// A route becomes reusable only after its corresponding lifecycle success.
func (s *routeStateStore) prepare(requestID, sessionKey string, route routeFingerprint, routeKnown bool, signals stateSignals) routeDecision {
	if s == nil {
		return failClosedDecision(signals, compactionPolicyBlock)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.purgeExpiredLocked(now)

	requestID = strings.TrimSpace(requestID)
	if requestID != "" {
		// CPA calls the after-auth hook for each retry candidate. Replace the
		// previous candidate for this one logical request instead of treating it
		// as independent concurrency.
		delete(s.pending, pendingKey(requestID))
	}

	decision := routeDecision{Relation: routeRelationUnknown}
	canTrack := sessionKey != "" && routeKnown
	if !canTrack {
		return failClosedDecision(signals, s.cfg.CompactionPolicy)
	}

	session, hasSession := s.sessions[sessionKey]
	hasOtherPending := s.hasPendingForSessionLocked(sessionKey)
	if s.hasConcurrentRouteMismatchLocked(sessionKey, route) {
		// Different credentials/models in flight for one conversation create an
		// ambiguous continuation order. Do not guess which encrypted state wins.
		session.Tainted = true
		session.UpdatedAt = now
		hasSession = true
	}

	if session.HasRoute && !session.Tainted {
		if session.Route.equal(route) {
			decision.Relation = routeRelationSame
		} else {
			decision.Relation = routeRelationChanged
		}
	} else if session.HasRoute {
		decision.Relation = routeRelationChanged
	}

	unsafeState := signals.hasRouteBoundState() && (session.Tainted || decision.Relation != routeRelationSame)
	if unsafeState && signals.HasCompaction && s.cfg.CompactionPolicy == compactionPolicyBlock {
		decision.Reject = true
		return decision
	}
	if unsafeState {
		decision.Strip = true
	}

	if requestID == "" {
		return decision
	}
	if !hasSession {
		session = sessionRouteState{}
	}
	session.UpdatedAt = now
	s.sessions[sessionKey] = session
	s.trimSessionsLocked()

	cleanStart := decision.Strip || !signals.hasRouteBoundState()
	s.sequence++
	s.pending[pendingKey(requestID)] = pendingRoute{
		SessionKey:    sessionKey,
		Route:         route,
		Sequence:      s.sequence,
		ObservedAt:    now,
		CleanStart:    cleanStart,
		CanClearTaint: session.Tainted && cleanStart && !hasOtherPending,
	}
	s.trimPendingLocked()
	return decision
}

func failClosedDecision(signals stateSignals, policy compactionPolicy) routeDecision {
	decision := routeDecision{Relation: routeRelationUnknown}
	if !signals.hasRouteBoundState() {
		return decision
	}
	if signals.HasCompaction && policy == compactionPolicyBlock {
		decision.Reject = true
		return decision
	}
	decision.Strip = true
	return decision
}

func (s *routeStateStore) complete(completion pluginapi.RequestCompletion) {
	if s == nil {
		return
	}
	requestID := strings.TrimSpace(completion.RequestID)
	if requestID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.purgeExpiredLocked(now)
	pending, exists := s.pending[pendingKey(requestID)]
	if !exists {
		return
	}
	delete(s.pending, pendingKey(requestID))

	current, existsCurrent := s.sessions[pending.SessionKey]
	if !existsCurrent {
		current = sessionRouteState{}
	}

	if completion.Outcome == pluginapi.RequestCompletionSucceeded {
		applied := false
		if !current.HasRoute || pending.Sequence >= current.Sequence {
			current.Route = pending.Route
			current.HasRoute = true
			current.Sequence = pending.Sequence
			current.UpdatedAt = now
			applied = true
		}
		// A tainted session can only become reusable after an unambiguous clean
		// request completed while no other request in that session was active.
		if pending.CanClearTaint && pending.CleanStart && applied && !s.hasPendingForSessionLocked(pending.SessionKey) {
			current.Tainted = false
		}
		s.sessions[pending.SessionKey] = current
		s.trimSessionsLocked()
		return
	}

	if !hasInvalidEncryptedContentError(completion.Error) {
		return
	}

	// The upstream has rejected a route-bound item despite the selected route.
	// Clear the binding and force a clean restart for the next continuation.
	current.Tainted = true
	current.UpdatedAt = now
	if current.HasRoute && current.Route.equal(pending.Route) && current.Sequence <= pending.Sequence {
		current.Route = routeFingerprint{}
		current.HasRoute = false
		current.Sequence = pending.Sequence
	}
	s.sessions[pending.SessionKey] = current
	s.trimSessionsLocked()
}

func (s *routeStateStore) reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequence = 0
	s.sessions = make(map[string]sessionRouteState)
	s.pending = make(map[string]pendingRoute)
}

func (s *routeStateStore) discardPending(requestID string) {
	if s == nil || strings.TrimSpace(requestID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, pendingKey(requestID))
}

func pendingKey(requestID string) string {
	return opaqueDigest("pending-v1", strings.TrimSpace(requestID))
}

func (s *routeStateStore) purgeExpiredLocked(now time.Time) {
	for key, state := range s.sessions {
		if now.Sub(state.UpdatedAt) > s.cfg.StateTTL {
			delete(s.sessions, key)
		}
	}
	for requestID, state := range s.pending {
		if now.Sub(state.ObservedAt) > s.cfg.StateTTL {
			delete(s.pending, requestID)
			s.taintSessionLocked(state.SessionKey, now)
		}
	}
}

func (s *routeStateStore) trimSessionsLocked() {
	for len(s.sessions) > s.cfg.MaxSessions {
		var oldestKey string
		var oldest sessionRouteState
		for key, state := range s.sessions {
			if oldestKey == "" || state.UpdatedAt.Before(oldest.UpdatedAt) || (state.UpdatedAt.Equal(oldest.UpdatedAt) && key < oldestKey) {
				oldestKey = key
				oldest = state
			}
		}
		delete(s.sessions, oldestKey)
	}
}

func (s *routeStateStore) trimPendingLocked() {
	for len(s.pending) > s.cfg.MaxPending {
		var oldestID string
		var oldest pendingRoute
		for requestID, state := range s.pending {
			if oldestID == "" || state.ObservedAt.Before(oldest.ObservedAt) || (state.ObservedAt.Equal(oldest.ObservedAt) && requestID < oldestID) {
				oldestID = requestID
				oldest = state
			}
		}
		delete(s.pending, oldestID)
		s.taintSessionLocked(oldest.SessionKey, s.now())
	}
}

func (s *routeStateStore) taintSessionLocked(sessionKey string, now time.Time) {
	if sessionKey == "" {
		return
	}
	state := s.sessions[sessionKey]
	state.Tainted = true
	state.UpdatedAt = now
	s.sessions[sessionKey] = state
	s.trimSessionsLocked()
}

func (s *routeStateStore) hasConcurrentRouteMismatchLocked(sessionKey string, route routeFingerprint) bool {
	for _, pending := range s.pending {
		if pending.SessionKey == sessionKey && !pending.Route.equal(route) {
			return true
		}
	}
	return false
}

func (s *routeStateStore) hasPendingForSessionLocked(sessionKey string) bool {
	for _, pending := range s.pending {
		if pending.SessionKey == sessionKey {
			return true
		}
	}
	return false
}

func hasInvalidEncryptedContentError(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "invalid_encrypted_content") ||
		strings.Contains(message, "thinking_signature_invalid") ||
		strings.Contains(message, "encrypted content could not be decrypted")
}
