// Package session checks the ZimaOS sign-in on every API request.
//
// The ZimaOS gateway forwards /v2/icedash/* without checking anything
// (measured on 1.8.0, docs/HOST-NOTES.md item 3), so the backend checks the
// token itself. The signature, issuer and expiry are checked by a Verifier
// (lintux-modkit's auth.Verifier on ZimaOS). Once a token passes, its payload
// can be trusted and gives the user's name and role: "admin" or "user"
// (measured, item 5).
package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/chicohaager/lintux-modkit/httpx"
)

// Verifier checks a raw token's signature, issuer and expiry.
type Verifier interface {
	Verify(token string) error
}

// Claims are the parts of the ZimaOS token IceDash uses.
type Claims struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Admin reports whether the user may change things.
func (c Claims) Admin() bool { return c.Role == "admin" }

type ctxKey struct{}

// FromContext returns the signed-in user of a request that passed Middleware.
func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(Claims)
	return c, ok
}

// WithClaims stores claims in ctx (Middleware does this; tests use it).
func WithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// Options adapt the checks to the system (PLAN.md, Phase 3a).
type Options struct {
	// System names the system in error messages ("ZimaOS" when empty).
	System string
	// EveryoneAdmin makes every signed-in user an admin. CasaOS has one
	// account per install and its token has no role claim (measured on 0.4.15).
	EveryoneAdmin bool
}

// Middleware lets a request through only with a valid token, and puts the
// user's claims in its context. It answers 401 with code auth_required (no
// token) or auth_invalid (a bad or expired one), so the UI can tell them apart.
func Middleware(v Verifier) func(http.Handler) http.Handler {
	return MiddlewareFor(v, Options{})
}

// MiddlewareFor is Middleware with the system's options.
func MiddlewareFor(v Verifier, o Options) func(http.Handler) http.Handler {
	system := o.System
	if system == "" {
		system = "ZimaOS"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := Token(r)
			if token == "" {
				httpx.WriteError(w, http.StatusUnauthorized, "auth_required", "sign in to "+system+" first")
				return
			}
			if err := v.Verify(token); err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "auth_invalid", "your "+system+" session has ended")
				return
			}
			c, err := Parse(token)
			if err != nil || c.Username == "" {
				httpx.WriteError(w, http.StatusUnauthorized, "auth_invalid", "your "+system+" session has ended")
				return
			}
			if o.EveryoneAdmin {
				c.Role = "admin"
			}
			next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), c)))
		})
	}
}

// RequireAdmin answers 403 admin_only unless the signed-in user is an admin.
// It must sit inside Middleware.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := FromContext(r.Context())
		if !ok || !c.Admin() {
			httpx.WriteError(w, http.StatusForbidden, "admin_only", "only an admin can do this")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Token reads the token from "Authorization: Bearer <token>" or the bare
// "Authorization: <token>" form; ZimaOS accepts both (measured, item 5).
func Token(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		h = strings.TrimSpace(h[7:])
	}
	return h
}

// Parse reads the claims from a token's payload without checking it. Only
// call it on a token a Verifier has accepted.
func Parse(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("not a JWT")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, err
	}
	var c Claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return Claims{}, err
	}
	return c, nil
}

// Disabled is a Verifier that accepts every token, for development on a PC
// without ZimaOS (ICEDASH_DISABLE_AUTH=1). The shipped unit never sets it.
type Disabled struct{}

// Verify accepts everything.
func (Disabled) Verify(string) error { return nil }
