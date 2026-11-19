package httpapi

import (
	"crypto/rsa"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"librasense/internal/store"
)

func (s *Server) deviceGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token := ""

		if len(header) > 7 && strings.EqualFold(header[:7], "bearer ") {
			token = strings.TrimSpace(header[7:])
		}

		hash, err := s.store.TokenHash(s.cfg.DeviceID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Kesalahan server"})
			return
		}

		if token == "" || !sameHash(store.HashToken(token), hash) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Token perangkat tidak valid"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func sameHash(left, right string) bool {
	if left == "" || len(left) != len(right) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func (s *Server) accessGuard(next http.Handler) http.Handler {
	if s.cfg.BypassAccess {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Cf-Access-Jwt-Assertion")

		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Butuh login Cloudflare Access"})
			return
		}

		key, err := s.keys.keyFor(token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Sesi Cloudflare Access tidak valid"})
			return
		}

		_, err = jwt.Parse(token, func(*jwt.Token) (any, error) { return key, nil },
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithIssuer(s.cfg.Access.TeamDomain),
			jwt.WithAudience(s.cfg.Access.Audience),
			jwt.WithExpirationRequired(),
		)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Sesi Cloudflare Access tidak valid"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

type jwksCache struct {
	mu      sync.Mutex
	teamURL string
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

func (c *jwksCache) keyFor(token string) (*rsa.PublicKey, error) {
	kid, err := tokenKeyID(token)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.keys) == 0 || time.Since(c.fetched) > time.Hour {
		if err := c.refresh(); err != nil {
			return nil, err
		}
	}

	if key, ok := c.keys[kid]; ok {
		return key, nil
	}

	if err := c.refresh(); err != nil {
		return nil, err
	}

	key, ok := c.keys[kid]
	if !ok {
		return nil, fmt.Errorf("kid %q tidak ditemukan", kid)
	}

	return key, nil
}

func (c *jwksCache) refresh() error {
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(c.teamURL + "/cdn-cgi/access/certs")
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks status %d", response.StatusCode)
	}

	var payload struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return err
	}

	keys := map[string]*rsa.PublicKey{}

	for _, item := range payload.Keys {
		if item.Kty != "RSA" || item.N == "" || item.E == "" {
			continue
		}

		modulus, err := base64.RawURLEncoding.DecodeString(item.N)
		if err != nil {
			continue
		}
		exponent, err := base64.RawURLEncoding.DecodeString(item.E)
		if err != nil {
			continue
		}

		value := new(big.Int).SetBytes(exponent)
		keys[item.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(modulus),
			E: int(value.Int64()),
		}
	}

	if len(keys) == 0 {
		return fmt.Errorf("jwks kosong")
	}

	c.keys = keys
	c.fetched = time.Now()

	return nil
}

func tokenKeyID(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("token tidak valid")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}

	var header struct {
		Kid string `json:"kid"`
	}

	if err := json.Unmarshal(payload, &header); err != nil {
		return "", err
	}

	if header.Kid == "" {
		return "", fmt.Errorf("kid tidak ada")
	}

	return header.Kid, nil
}
