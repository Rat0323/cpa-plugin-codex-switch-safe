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
	}{
		{name: "CPA 7.2.130", schemaVersion: 3},
		{name: "CPA 7.2.145", schemaVersion: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, errMarshal := json.Marshal(abiLifecycleRequest{SchemaVersion: tc.schemaVersion})
			if errMarshal != nil {
				t.Fatal(errMarshal)
			}
			raw, errRegister := handleABIMethod(context.Background(), pluginabi.MethodPluginRegister, request)
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
			if result.SchemaVersion != tc.schemaVersion {
				t.Fatalf("schema_version = %d, want %d", result.SchemaVersion, tc.schemaVersion)
			}
			if !result.Capabilities.RequestInterceptor || !result.Capabilities.RequestLifecyclePlugin {
				t.Fatalf("capabilities = %#v", result.Capabilities)
			}
		})
	}
}
