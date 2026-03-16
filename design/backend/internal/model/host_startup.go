package model

import "strings"

const (
	HostStartupProfileDisabled = "disabled"
	HostStartupProfileStocker  = "stocker"
	HostStartupProfileConveyor = "conveyor"
)

func NormalizeHostStartupProfile(profile string, legacyEnabled bool) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", "default":
		if legacyEnabled {
			return HostStartupProfileStocker
		}
		return HostStartupProfileDisabled
	case "disabled", "none", "off":
		return HostStartupProfileDisabled
	case "stocker", "minimal", "stocker-minimal":
		return HostStartupProfileStocker
	case "conveyor", "conveyor-example", "example-conveyor":
		return HostStartupProfileConveyor
	default:
		if legacyEnabled {
			return HostStartupProfileStocker
		}
		return HostStartupProfileDisabled
	}
}

func NormalizedHostStartupProfile(handshake HandshakeConfig) string {
	return NormalizeHostStartupProfile(handshake.HostStartupProfile, handshake.AutoHostStartup)
}

func HostStartupEnabled(handshake HandshakeConfig) bool {
	return NormalizedHostStartupProfile(handshake) != HostStartupProfileDisabled
}
