package youtube

import "sync"

var (
	apiRequestGuardMu sync.RWMutex
	apiRequestGuard   func() bool
)

// SetAPIRequestGuard installs the application's daily YouTube Data API request
// budget guard. Returning false prevents a quota-consuming API request and lets
// callers fall back to the public/Innertube transport.
func SetAPIRequestGuard(guard func() bool) {
	apiRequestGuardMu.Lock()
	apiRequestGuard = guard
	apiRequestGuardMu.Unlock()
}

// TryConsumeAPIRequest reserves one YouTube Data API request. It is intentionally
// called immediately before each API request so the application's daily budget
// cannot be exceeded by concurrent connections.
func TryConsumeAPIRequest() bool {
	apiRequestGuardMu.RLock()
	guard := apiRequestGuard
	apiRequestGuardMu.RUnlock()

	if guard == nil {
		return true
	}
	return guard()
}
