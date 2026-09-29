package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr    string
	Env         string
	LogLevel    string
	CORSOrigins []string

	DB DBConfig

	JWT       JWTConfig
	Cookie    CookieConfig
	SuperUser SuperUserConfig
	Session   SessionConfig

	PublicFormBaseURL string

	// PublicAPIBaseURL is this API's own externally-reachable base URL (e.g.
	// https://studio.example.com/api in prod, behind nginx). Used to build
	// per-channel webhook URLs we register with third parties at connect
	// time (currently just Telegram's setWebhook — Meta/Twilio/X webhook
	// URLs are configured once, manually, in their respective dashboards).
	PublicAPIBaseURL string

	Sheets SheetsConfig

	// Encryption key for at-rest secrets (channel access tokens). 32-byte
	// AES-256 key, base64-encoded. Generate with: openssl rand -base64 32
	TokenEncryptionKey string

	// Meta App credentials (single platform-wide app; each studio brings its
	// own WABA + access token via the Channels page).
	Meta   MetaConfig
	Claude ClaudeConfig
	Groq   GroqConfig
	S3     S3Config
	Glofox GlofoxConfig
	Redis  RedisConfig
	SMTP   SMTPConfig
}

// SMTPConfig sends transactional email (currently just password-reset
// links). Unset (Host empty) means email sending is disabled — Enabled()
// gates every call site so a studio/dev environment without SMTP configured
// degrades to "reset link couldn't be emailed" rather than a boot failure.
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

func (s SMTPConfig) Enabled() bool {
	return s.Host != "" && s.User != "" && s.Password != ""
}

// RedisConfig backs the per-studio AI-answer cache (internal/messaging.AnswerCache).
// Redis being unreachable degrades to always calling the model — it is
// never required for the app to boot or serve requests. Discrete
// host/port/password fields (not a redis://user:pass@host URL) so a
// password containing URL-special characters is never mis-parsed.
type RedisConfig struct {
	Host           string
	Port           int
	Password       string
	DB             int
	AnswerCacheTTL time.Duration
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type GlofoxConfig struct {
	APIKey   string
	APIToken string
	BranchID string // Glofox _id of the branch/location (x-glofox-branch-id)
}

func (g GlofoxConfig) Enabled() bool { return g.APIKey != "" && g.APIToken != "" && g.BranchID != "" }

type MetaConfig struct {
	AppID              string
	AppSecret          string
	WebhookVerifyToken string
	GraphAPIVersion    string // e.g. "v21.0"
}

type ClaudeConfig struct {
	APIURL string
	APIKey string
}

type GroqConfig struct {
	APIKey string
}

type S3Config struct {
	Region        string
	AccessKeyID   string
	SecretKey     string
	Bucket        string
	PublicURLBase string
}

func (s S3Config) Enabled() bool {
	return s.Region != "" && s.AccessKeyID != "" && s.SecretKey != "" && s.Bucket != ""
}

func (m MetaConfig) Enabled() bool {
	return m.AppSecret != "" && m.WebhookVerifyToken != ""
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

func (d DBConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode)
}

type JWTConfig struct {
	Secret string
	TTL    time.Duration
}

type CookieConfig struct {
	Name   string
	Domain string
	Secure bool
}

// SessionConfig backs identity.SessionStore — the Redis-tracked-by-jti
// session that enforces revocation and an idle timeout shorter than the
// JWT's own absolute TTL (JWTConfig.TTL). See SessionStore's doc comment:
// unlike RedisConfig's other consumer (the AI answer cache), Redis being
// unreachable here fails closed (requests are rejected), since this is an
// access-control mechanism, not a perf cache.
type SessionConfig struct {
	IdleTimeout time.Duration
}

type SuperUserConfig struct {
	Email    string
	Password string
}

type SheetsConfig struct {
	CredentialsPath string
	SpreadsheetID   string
	Tab             string
}

func (s SheetsConfig) Enabled() bool {
	return s.CredentialsPath != "" && s.SpreadsheetID != ""
}

// Load reads .env (if present) then merges OS env. Fails fast on missing
// required values so misconfiguration is loud.
func Load() (Config, error) {
	for _, path := range []string{".env", "../.env", "../../.env"} {
		_ = godotenv.Load(path)
	}

	port, err := atoiDefault("POSTGRES_PORT", 5432)
	if err != nil {
		return Config{}, err
	}
	jwtTTL, err := durationDefault("JWT_TTL", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	answerCacheTTL, err := durationDefault("AI_ANSWER_CACHE_TTL", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	sessionIdleTimeout, err := durationDefault("SESSION_IDLE_TIMEOUT", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	redisPort, err := atoiDefault("REDIS_PORT", 6379)
	if err != nil {
		return Config{}, err
	}
	redisDB, err := atoiDefault("REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}
	smtpPort, err := atoiDefault("SMTP_PORT", 587)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:    getEnv("API_HTTP_ADDR", "localhost:8080"),
		Env:         getEnv("API_ENV", "local"),
		LogLevel:    getEnv("API_LOG_LEVEL", "info"),
		CORSOrigins: splitCSV(getEnv("API_CORS_ORIGINS", "http://localhost:3000,http://localhost:3001")),
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", ""),
			TTL:    jwtTTL,
		},
		Cookie: CookieConfig{
			Name:   getEnv("COOKIE_NAME", "px_session"),
			Domain: os.Getenv("COOKIE_DOMAIN"),
			Secure: getEnv("COOKIE_SECURE", "false") == "true",
		},
		SuperUser: SuperUserConfig{
			Email:    getEnv("SUPER_ADMIN_EMAIL", ""),
			Password: getEnv("SUPER_ADMIN_PASSWORD", ""),
		},
		Session: SessionConfig{
			IdleTimeout: sessionIdleTimeout,
		},
		PublicFormBaseURL: getEnv("PUBLIC_FORM_BASE_URL", "http://localhost:3000"),
		PublicAPIBaseURL:  getEnv("PUBLIC_API_BASE_URL", "http://localhost:8080"),
		Sheets: SheetsConfig{
			CredentialsPath: getEnv("GOOGLE_CREDENTIALS_PATH", ""),
			SpreadsheetID:   getEnv("GOOGLE_SHEETS_ID", ""),
			Tab:             getEnv("GOOGLE_SHEETS_TAB", "Leads"),
		},
		TokenEncryptionKey: getEnv("TOKEN_ENCRYPTION_KEY", ""),
		Meta: MetaConfig{
			AppID:              getEnv("META_APP_ID", ""),
			AppSecret:          getEnv("META_APP_SECRET", ""),
			WebhookVerifyToken: getEnv("META_WEBHOOK_VERIFY_TOKEN", ""),
			GraphAPIVersion:    getEnv("META_GRAPH_API_VERSION", "v21.0"),
		},
		Claude: ClaudeConfig{
			APIURL: getEnv("CLAUDE_API_URL", "https://api.anthropic.com/v1/messages"),
			APIKey: getEnv("CLAUDE_API_KEY", ""),
		},
		Groq: GroqConfig{
			APIKey: getEnv("GROQ_API_KEY", ""),
		},
		S3: S3Config{
			Region:        getEnv("AWS_REGION", ""),
			AccessKeyID:   getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretKey:     getEnv("AWS_SECRET_ACCESS_KEY", ""),
			Bucket:        getEnv("S3_BUCKET", ""),
			PublicURLBase: getEnv("S3_PUBLIC_URL", ""),
		},
		Glofox: GlofoxConfig{
			APIKey:   getEnv("GLOFOX_API_KEY", ""),
			APIToken: getEnv("GLOFOX_API_TOKEN", ""),
			BranchID: getEnv("GLOFOX_BRANCH_ID", ""),
		},
		Redis: RedisConfig{
			Host:           getEnv("REDIS_HOST", "localhost"),
			Port:           redisPort,
			Password:       getEnv("REDIS_PASSWORD", ""),
			DB:             redisDB,
			AnswerCacheTTL: answerCacheTTL,
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", ""),
			Port:     smtpPort,
			User:     getEnv("SMTP_USER", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", getEnv("SMTP_USER", "")),
		},
	}

	dbCfg, err := loadDBConfig(port)
	if err != nil {
		return Config{}, err
	}
	cfg.DB = dbCfg

	if len(cfg.JWT.Secret) < 32 {
		return cfg, errors.New("JWT_SECRET must be at least 32 characters")
	}
	if cfg.TokenEncryptionKey == "" {
		return cfg, errors.New("TOKEN_ENCRYPTION_KEY is required (generate with `openssl rand -base64 32`)")
	}
	return cfg, nil
}

func loadDBConfig(defaultPort int) (DBConfig, error) {
	if rawURL := os.Getenv("DATABASE_URL"); rawURL != "" {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return DBConfig{}, fmt.Errorf("DATABASE_URL: %w", err)
		}
		password, _ := parsed.User.Password()
		port, err := strconv.Atoi(parsed.Port())
		if err != nil {
			port = defaultPort
		}
		sslMode := parsed.Query().Get("sslmode")
		if sslMode == "" {
			sslMode = "disable"
		}
		return DBConfig{
			Host:     parsed.Hostname(),
			Port:     port,
			User:     parsed.User.Username(),
			Password: password,
			Name:     strings.TrimPrefix(parsed.Path, "/"),
			SSLMode:  sslMode,
		}, nil
	}

	return DBConfig{
		Host:     getEnv("POSTGRES_HOST", "localhost"),
		Port:     defaultPort,
		User:     getEnv("POSTGRES_USER", "projectx"),
		Password: getEnv("POSTGRES_PASSWORD", ""),
		Name:     getEnv("POSTGRES_DB", "projectx"),
		SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
	}, nil
}

func getEnv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func atoiDefault(k string, def int) (int, error) {
	v, ok := os.LookupEnv(k)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("env %s: %w", k, err)
	}
	return n, nil
}

func durationDefault(k string, def time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(k)
	if !ok || v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("env %s: %w", k, err)
	}
	return d, nil
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
