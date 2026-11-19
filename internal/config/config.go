package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Access struct {
	TeamDomain string
	Audience   string
}

type Config struct {
	Env          string
	Port         int
	DataDir      string
	DeviceID     string
	DeviceToken  string
	BypassAccess bool
	Access       *Access
}

func Load() (*Config, error) {
	env := value("APP_ENV", "development")

	switch env {
	case "development", "test", "production":
	default:
		return nil, fmt.Errorf("APP_ENV harus development, test, atau production")
	}

	port, err := strconv.Atoi(value("PORT", "8787"))
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("PORT harus angka 1 sampai 65535")
	}

	token := os.Getenv("DEVICE_TOKEN")
	if len(token) < 16 {
		return nil, fmt.Errorf("DEVICE_TOKEN wajib diisi minimal 16 karakter")
	}

	bypass := value("ACCESS_DEV_BYPASS", "false") == "true"

	if env == "production" && bypass {
		return nil, fmt.Errorf("ACCESS_DEV_BYPASS tidak boleh aktif saat APP_ENV=production")
	}

	teamDomain := strings.TrimSpace(os.Getenv("CF_ACCESS_TEAM_DOMAIN"))
	audience := strings.TrimSpace(os.Getenv("CF_ACCESS_AUD"))

	if !bypass && (teamDomain == "" || audience == "") {
		return nil, fmt.Errorf("CF_ACCESS_TEAM_DOMAIN dan CF_ACCESS_AUD wajib diisi, atau set ACCESS_DEV_BYPASS=true untuk lokal")
	}

	cfg := &Config{
		Env:          env,
		Port:         port,
		DataDir:      value("DATA_DIR", "./data"),
		DeviceID:     value("DEVICE_ID", "perpustakaan-1"),
		DeviceToken:  token,
		BypassAccess: bypass,
	}

	if !bypass {
		domain, err := normalizeDomain(teamDomain)
		if err != nil {
			return nil, err
		}
		cfg.Access = &Access{TeamDomain: domain, Audience: audience}
	}

	return cfg, nil
}

func value(key, fallback string) string {
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		return raw
	}
	return fallback
}

func normalizeDomain(raw string) (string, error) {
	raw = strings.TrimRight(raw, "/")
	if !strings.HasPrefix(raw, "http") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("CF_ACCESS_TEAM_DOMAIN tidak valid")
	}

	return parsed.Scheme + "://" + parsed.Host, nil
}
