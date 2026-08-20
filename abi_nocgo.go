//go:build !cgo

package main

// Unit tests and static checks do not have a host ABI to receive diagnostics.
func hostDiagnosticSink(_ string, _ string, _ map[string]any) {}
