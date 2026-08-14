package main

import "github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"

type abiRegistration struct {
	SchemaVersion uint32             `json:"schema_version"`
	Metadata      pluginapi.Metadata `json:"metadata"`
	Capabilities  abiCapabilities    `json:"capabilities"`
}

type abiCapabilities struct {
	RequestInterceptor     bool `json:"request_interceptor"`
	RequestLifecyclePlugin bool `json:"request_lifecycle_plugin"`
}

func pluginRegistration() abiRegistration {
	return abiRegistration{
		SchemaVersion: pluginSchemaVersion,
		Metadata:      pluginMetadata(),
		Capabilities: abiCapabilities{
			RequestInterceptor:     true,
			RequestLifecyclePlugin: true,
		},
	}
}
