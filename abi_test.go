//go:build cgo

package main

import (
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
)

func TestABIRegistrationAdvertisesLifecycleAndSchema(t *testing.T) {
	raw, errMarshal := abiOKEnvelope(pluginRegistration())
	if errMarshal != nil {
		t.Fatal(errMarshal)
	}
	var envelope abiEnvelope
	if errUnmarshal := json.Unmarshal(raw, &envelope); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	var result abiRegistration
	if errUnmarshal := json.Unmarshal(envelope.Result, &result); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	if result.SchemaVersion != pluginabi.SchemaVersion {
		t.Fatalf("schema_version = %d, want %d", result.SchemaVersion, pluginabi.SchemaVersion)
	}
	if !result.Capabilities.RequestInterceptor || !result.Capabilities.RequestLifecyclePlugin {
		t.Fatalf("capabilities = %#v", result.Capabilities)
	}
}
