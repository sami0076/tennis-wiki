package httpapi

import (
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		wantErr []string
		check   func(*testing.T, Config)
	}{
		{
			name: "defaults",
			env:  map[string]string{"DATABASE_URL": "postgres://localhost/x"},
			check: func(t *testing.T, c Config) {
				if c.Addr != ":8080" || c.RateLimitPerMin != 60 || c.TrustProxy {
					t.Errorf("unexpected defaults: %+v", c)
				}
			},
		},
		{
			name:    "the database url is required",
			env:     map[string]string{},
			wantErr: []string{"DATABASE_URL"},
		},
		{
			name: "several problems are reported at once",
			env: map[string]string{
				"API_RATE_LIMIT":   "not-a-number",
				"API_TRUST_PROXY":  "maybe",
				"API_CORS_ORIGINS": "*",
			},
			wantErr: []string{"DATABASE_URL", "API_RATE_LIMIT", "API_TRUST_PROXY", "API_CORS_ORIGINS"},
		},
		{
			name: "a wildcard origin is refused",
			env: map[string]string{
				"DATABASE_URL":     "postgres://localhost/x",
				"API_CORS_ORIGINS": "*",
			},
			wantErr: []string{`may not be "*"`},
		},
		{
			name: "zero rate limit is refused",
			env: map[string]string{
				"DATABASE_URL":   "postgres://localhost/x",
				"API_RATE_LIMIT": "0",
			},
			wantErr: []string{"at least 1"},
		},
		{
			name: "origins are split and trimmed",
			env: map[string]string{
				"DATABASE_URL":     "postgres://localhost/x",
				"API_CORS_ORIGINS": "https://a.example, https://b.example",
			},
			check: func(t *testing.T, c Config) {
				want := []string{"https://a.example", "https://b.example"}
				if len(c.CORSOrigins) != 2 || c.CORSOrigins[0] != want[0] || c.CORSOrigins[1] != want[1] {
					t.Errorf("origins = %q, want %q", c.CORSOrigins, want)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, key := range []string{"API_ADDR", "DATABASE_URL", "API_CORS_ORIGINS",
				"API_RATE_LIMIT", "API_TRUST_PROXY"} {
				t.Setenv(key, "")
			}
			for k, v := range c.env {
				t.Setenv(k, v)
			}

			cfg, err := LoadConfig()
			if len(c.wantErr) == 0 {
				if err != nil {
					t.Fatalf("LoadConfig: %v", err)
				}
				c.check(t, cfg)
				return
			}
			if err == nil {
				t.Fatalf("expected an error mentioning %v", c.wantErr)
			}
			for _, want := range c.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}
