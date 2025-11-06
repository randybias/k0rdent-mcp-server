//go:build !live

package integration

import "testing"

func TestIntegrationPlaceholder(t *testing.T) {
    t.Skip("integration tests require build tag 'live'")
}
