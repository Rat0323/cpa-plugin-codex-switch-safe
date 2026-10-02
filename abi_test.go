//go:build cgo

package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
)

func TestABIRegistrationAdvertisesLifecycleAndSchema(t *testing.T) {
	t.Cleanup(func() {
		switchSafeABIState.Lock()
		switchSafeABIState.plugin = nil
		switchSafeABIState.Unlock()
	})

	for _, tc := range []struct {
		name          string
		schemaVersion uint32
		wantVersion   uint32
		wantError     bool
	}{
		{name: "CPA 7.2.130 schema 3", schemaVersion: 3, wantVersion: 3},
		{name: "CPA 7.2.145 schema 4", schemaVersion: 4, wantVersion: 4},
		{name: "CPA 7.2.151 schema 5", schemaVersion: 5, wantVersion: 5},
		{name: "current schema", schemaVersion: pluginSchemaVersion, wantVersion: pluginSchemaVersion},
		{name: "future schema", schemaVersion: pluginSchemaVersion + 1, wantVersion: pluginSchemaVersion},
		{name: "schema 0", schemaVersion: 0, wantError: true},
		{name: "schema 1", schemaVersion: 1, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, errMarshal := json.Marshal(abiLifecycleRequest{SchemaVersion: tc.schemaVersion})
			if errMarshal != nil {
				t.Fatal(errMarshal)
			}
			raw, errRegister := handleABIMethod(context.Background(), pluginabi.MethodPluginRegister, request)
			if tc.wantError {
				if errRegister == nil {
					t.Fatalf("register schema %d succeeded, want error", tc.schemaVersion)
				}
				return
			}
			if errRegister != nil {
				t.Fatalf("register schema %d: %v", tc.schemaVersion, errRegister)
			}

			var envelope abiEnvelope
			if errUnmarshal := json.Unmarshal(raw, &envelope); errUnmarshal != nil {
				t.Fatal(errUnmarshal)
			}
			var result abiRegistration
			if errUnmarshal := json.Unmarshal(envelope.Result, &result); errUnmarshal != nil {
				t.Fatal(errUnmarshal)
			}
			if result.SchemaVersion != tc.wantVersion {
				t.Fatalf("schema_version = %d, want %d", result.SchemaVersion, tc.wantVersion)
			}
			if !result.Capabilities.RequestInterceptor || !result.Capabilities.RequestLifecyclePlugin {
				t.Fatalf("capabilities = %#v", result.Capabilities)
			}
		})
	}
}
