package main

import (
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

var pluginVersion = "0.1.1"

const pluginSchemaVersion = pluginabi.SchemaVersion

func buildPlugin(configYAML []byte) (*switchSafePlugin, error) {
	cfg, errParse := parsePluginConfig(configYAML)
	if errParse != nil {
		return nil, errParse
	}
	return &switchSafePlugin{
		cfg:   cfg,
		state: newRouteStateStore(cfg),
	}, nil
}

func pluginMetadata() pluginapi.Metadata {
	return pluginapi.Metadata{
		Name:             pluginName,
		Version:          pluginVersion,
		Author:           "Rat0323",
		GitHubRepository: "https://github.com/Rat0323/cpa-plugin-codex-switch-safe",
		ConfigFields: []pluginapi.ConfigField{
			{
				Name:        "state_ttl",
				Type:        pluginapi.ConfigFieldTypeString,
				Description: "In-memory route-state lifetime, from 1m through 24h. Default: 4h.",
			},
			{
				Name:        "max_sessions",
				Type:        pluginapi.ConfigFieldTypeInteger,
				Description: "Maximum successful conversation route entries retained in memory. Default: 4096.",
			},
			{
				Name:        "max_pending",
				Type:        pluginapi.ConfigFieldTypeInteger,
				Description: "Maximum selected-auth attempts retained while requests are in flight. Default: 8192.",
			},
			{
				Name:        "compaction_policy",
				Type:        pluginapi.ConfigFieldTypeEnum,
				EnumValues:  []string{string(compactionPolicyBlock), string(compactionPolicyStrip)},
				Description: "block preserves long-context correctness on route changes; strip continues after dropping route-bound compaction state.",
			},
		},
	}
}

func main() {}
