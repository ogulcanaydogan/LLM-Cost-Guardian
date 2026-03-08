package httpauth

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	keyauth "github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/auth"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/storage"
)

type contextKey string

const identityKey contextKey = "lcg.identity"

// Identity captures the authenticated tenant and access mode for a request.
type Identity struct {
	Tenant model.Tenant
	APIKey *model.APIKey
	Admin  bool
}

// Middleware authenticates HTTP requests and injects tenant identity into context.
type Middleware struct {
	store        storage.Storage
	enabled      bool
	defaultTenant string
	bootstrapKey string
	logger       *slog.Logger
}

// New creates a tenant auth middleware.
func New(store storage.Storage, enabled bool, defaultTenant, bootstrapKey string, logger *slog.Logger) *Middleware {
	if strings.TrimSpace(defaultTenant) == "" {
		defaultTenant = "default"
	}
	return &Middleware{
		store:         store,
		enabled:       enabled,
		defaultTenant: strings.TrimSpace(defaultTenant),
		bootstrapKey:  strings.TrimSpace(bootstrapKey),
		logger:        logger,
	}
}

// Wrap applies tenant authentication to a handler.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, err := m.authenticate(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), identity)))
	})
}

// WithIdentity stores identity in a context.
func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}

// IdentityFromContext returns the request identity if present.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityKey).(Identity)
	return identity, ok
}

func (m *Middleware) authenticate(r *http.Request) (Identity, error) {
	if !m.enabled {
		tenant, err := m.store.EnsureTenant(r.Context(), m.defaultTenant, "Default")
		if err != nil {
			return Identity{}, err
		}
		return Identity{Tenant: *tenant, Admin: true}, nil
	}

	rawKey := keyauth.ExtractAPIKey(r)
	if rawKey == "" {
		return Identity{}, http.ErrNoCookie
	}

	if m.bootstrapKey != "" && subtle.ConstantTimeCompare([]byte(rawKey), []byte(m.bootstrapKey)) == 1 {
		tenant, err := m.store.EnsureTenant(r.Context(), m.defaultTenant, "Default")
		if err != nil {
			return Identity{}, err
		}
		return Identity{Tenant: *tenant, Admin: true}, nil
	}

	key, tenant, err := m.store.ResolveAPIKey(r.Context(), keyauth.HashAPIKey(rawKey))
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("api key authentication failed", "error", err)
		}
		return Identity{}, err
	}

	return Identity{
		Tenant: *tenant,
		APIKey: key,
	}, nil
}
