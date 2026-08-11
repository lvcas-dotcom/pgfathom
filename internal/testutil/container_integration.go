//go:build integration || benchmark

// Everything behind this build tag needs Docker. Keeping it tagged is what lets
// `go test ./...` stay fast and offline, so contributing a naming profile never
// requires installing a container runtime.
//
// Phase 2 fills this in with a testcontainers-backed PostgreSQL fixture.
package testutil

// IntegrationEnabled reports whether the integration build tag is active.
const IntegrationEnabled = true
