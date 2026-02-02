package main

import "os"

type FeatureFlags struct {
	FaucetsEnabled bool
	SinksEnabled   bool
	Telemetry      bool
}

var featureFlags = loadFeatureFlags()

func loadFeatureFlags() FeatureFlags {
	return FeatureFlags{
		FaucetsEnabled: envFlag("ENABLE_FAUCETS", true),
		SinksEnabled:   envFlag("ENABLE_SINKS", true),
		Telemetry:      envFlag("ENABLE_TELEMETRY", true),
	}
}

func envFlag(name string, fallback bool) bool {
	val := os.Getenv(name)
	if val == "" {
		return fallback
	}
	return val == "true" || val == "1" || val == "yes"
}
