package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
)

// ErrUnauthorized is returned when a request is missing required authentication.
var ErrUnauthorized = errors.New("unauthorized")

// Gate validates incoming HTTP requests according to the configured AUTH_MODE.
type Gate struct {
	mode   config.AuthMode
	logger *slog.Logger
}

// NewGate creates an authorization gate for the provided mode.
func NewGate(mode config.AuthMode, logger *slog.Logger) *Gate {
	if logger == nil {
		logger = slog.Default()
	}
	return &Gate{
		mode:   mode,
		logger: logging.WithComponent(logger, "auth.gate"),
	}
}

// ExtractBearer validates the Authorization header and returns the bearer token, if any.
//
// When the mode is AuthModeOIDCRequired, a missing or malformed bearer token results in an error.
// For AuthModeDevAllowAny, requests are accepted even when the header is missing, but malformed
// Authorization headers still return an error so clients fix their requests.
func (g *Gate) ExtractBearer(r *http.Request) (string, error) {
	log := logging.WithContext(r.Context(), g.logger)

	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if authz == "" {
		if g.mode == config.AuthModeOIDCRequired {
			if log != nil {
				log.Warn("authorization header missing")
			}
			return "", ErrUnauthorized
		}
		if log != nil {
			log.Debug("authorization optional and missing")
		}
		return "", nil
	}

	const prefix = "Bearer "
	if len(authz) < len(prefix) || !strings.EqualFold(authz[:len(prefix)], prefix) {
		if log != nil {
			log.Warn("authorization header must use bearer scheme")
		}
		return "", errors.New("authorization header must use Bearer scheme")
	}

	token := strings.TrimSpace(authz[len(prefix):])
	if token == "" {
		if log != nil {
			log.Warn("authorization header missing bearer token")
		}
		return "", errors.New("authorization header missing bearer token")
	}

	if log != nil {
		log.Debug("authorization header accepted", "requires_auth", g.RequiresAuth())
	}

	return token, nil
}

// RequiresAuth reports whether the gate requires an Authorization header.
func (g *Gate) RequiresAuth() bool {
	return g != nil && g.mode == config.AuthModeOIDCRequired
}
