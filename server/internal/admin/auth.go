package admin

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AccessHeader is the header the access gate puts its signed identity
// assertion in.
const AccessHeader = "Cf-Access-Jwt-Assertion"

// Identity is who the access gate says is making the request.
type Identity struct {
	Email string
}

// Verifier checks the signed identity assertion supplied by the access gate.
// The admin API refuses requests without a valid one, so it stays safe even
// if the gate in front of it is misconfigured or bypassed.
type Verifier struct {
	issuer   string // https://<team domain>
	audience string
	keys     KeySource
	now      func() time.Time
}

// KeySource returns the gate's current public signing keys, by key id.
type KeySource interface {
	Keys(ctx context.Context) (map[string]*rsa.PublicKey, error)
}

// NewVerifier builds a verifier for the given team domain and audience tag.
// keys may be nil, in which case keys are fetched from the team's published
// certificate URL and cached.
func NewVerifier(teamDomain, audience string, keys KeySource) (*Verifier, error) {
	teamDomain = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(teamDomain, "https://"), "http://"), "/")
	if teamDomain == "" || audience == "" {
		return nil, errors.New("admin: access team domain and audience are both required")
	}
	if keys == nil {
		keys = &remoteKeys{url: "https://" + teamDomain + "/cdn-cgi/access/certs", client: &http.Client{Timeout: 10 * time.Second}}
	}
	return &Verifier{issuer: "https://" + teamDomain, audience: audience, keys: keys, now: time.Now}, nil
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

type jwtClaims struct {
	Issuer    string          `json:"iss"`
	Audience  json.RawMessage `json:"aud"`
	Expires   float64         `json:"exp"`
	NotBefore float64         `json:"nbf"`
	Email     string          `json:"email"`
}

// Verify validates the token's signature, issuer, audience and times.
func (v *Verifier) Verify(ctx context.Context, token string) (Identity, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Identity{}, errors.New("malformed token")
	}
	var hdr jwtHeader
	if err := decodePart(parts[0], &hdr); err != nil {
		return Identity{}, err
	}
	if hdr.Alg != "RS256" { // never accept "none" or symmetric algorithms
		return Identity{}, errors.New("unsupported algorithm")
	}
	keys, err := v.keys.Keys(ctx)
	if err != nil {
		return Identity{}, fmt.Errorf("load keys: %w", err)
	}
	key, ok := keys[hdr.Kid]
	if !ok {
		return Identity{}, errors.New("unknown signing key")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Identity{}, errors.New("malformed signature")
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig); err != nil {
		return Identity{}, errors.New("bad signature")
	}
	var c jwtClaims
	if err := decodePart(parts[1], &c); err != nil {
		return Identity{}, err
	}
	now := float64(v.now().Unix())
	if c.Issuer != v.issuer {
		return Identity{}, errors.New("wrong issuer")
	}
	if !audienceMatches(c.Audience, v.audience) {
		return Identity{}, errors.New("wrong audience")
	}
	if c.Expires == 0 || now >= c.Expires {
		return Identity{}, errors.New("token expired")
	}
	if c.NotBefore != 0 && now < c.NotBefore-30 {
		return Identity{}, errors.New("token not yet valid")
	}
	return Identity{Email: c.Email}, nil
}

func decodePart(s string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return errors.New("malformed token")
	}
	if err := json.Unmarshal(b, v); err != nil {
		return errors.New("malformed token")
	}
	return nil
}

func audienceMatches(raw json.RawMessage, want string) bool {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one == want
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		for _, a := range many {
			if a == want {
				return true
			}
		}
	}
	return false
}

// remoteKeys fetches and caches the gate's published signing keys.
type remoteKeys struct {
	url    string
	client *http.Client

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

func (r *remoteKeys) Keys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.keys != nil && time.Since(r.fetched) < time.Hour {
		return r.keys, nil
	}
	// Refetch at most once a minute even on failure/unknown kid churn.
	if r.keys != nil && time.Since(r.fetched) < time.Minute {
		return r.keys, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		if r.keys != nil {
			return r.keys, nil // keep serving with the last good keys
		}
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if r.keys != nil {
			return r.keys, nil
		}
		return nil, fmt.Errorf("key fetch: status %d", resp.StatusCode)
	}
	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("key fetch: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return nil, errors.New("key fetch: no usable keys")
	}
	r.keys, r.fetched = keys, time.Now()
	return keys, nil
}

type ctxKey struct{}

// IdentityFrom returns the authenticated identity placed in ctx by the
// authentication middleware.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// Authenticate requires a valid access assertion on every request. If dev is
// true the check is skipped and a fixed local identity is used; the caller is
// responsible for only allowing that on a loopback listener.
func Authenticate(v *Verifier, dev bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if dev {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, Identity{Email: "dev@localhost"})))
				return
			}
			tok := r.Header.Get(AccessHeader)
			if v == nil || tok == "" {
				writeError(w, http.StatusUnauthorized, "Please sign in to make changes.")
				return
			}
			id, err := v.Verify(r.Context(), tok)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "Please sign in to make changes.")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
		})
	}
}
