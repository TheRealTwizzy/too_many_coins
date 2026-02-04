package main

import (
	"fmt"
	"os"
	"strings"
)

type Phase string

const (
	PhaseAlpha   Phase = "alpha"
	PhaseBeta    Phase = "beta"
	PhaseRelease Phase = "release"
)

// PhaseFromEnv returns the server-defined phase. It only reads server env vars
// (PHASE with APP_ENV fallback) and rejects unknown values.
func PhaseFromEnv() (Phase, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("PHASE")))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	}
	switch value {
	case string(PhaseAlpha):
		return PhaseAlpha, nil
	case string(PhaseBeta):
		return PhaseBeta, nil
	case string(PhaseRelease):
		return PhaseRelease, nil
	case "":
		return "", fmt.Errorf("phase is required (set PHASE or APP_ENV to alpha, beta, or release)")
	default:
		return "", fmt.Errorf("unsupported phase: %s", value)
	}
}
