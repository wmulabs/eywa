package fiber

import (
	"io"
	"net/http"
	"net/http/httptest"
)

// testAdminKey is the API key used by tests that exercise authenticated routes.
const testAdminKey = "test-admin-key"

// authedAPIKeys is the RouteDeps auth config those tests register with.
func authedAPIKeys() map[string]string {
	return map[string]string{testAdminKey: "admin"}
}

// authedRequest builds an httptest request carrying a valid bearer token.
func authedRequest(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.Header.Set("Authorization", "Bearer "+testAdminKey)
	return r
}
