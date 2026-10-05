package studio

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Config struct{ DatabaseURL, MasterKey, Origin, DataDir, WebDir, FontPath, Addr string }

func ConfigFromEnv() Config {
	return Config{os.Getenv("DATABASE_URL"), os.Getenv("APP_MASTER_KEY"), env("APP_ORIGIN", "http://localhost:8080"), env("DATA_DIR", "/data"), env("WEB_DIR", "/app/web"), env("FONT_PATH", "/app/fonts/NotoSansSC.ttf"), env("LISTEN_ADDR", ":8080")}
}
func env(key, fallback string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return fallback
}

type App struct {
	DB        *pgxpool.Pool
	Cfg       Config
	cipher    cipher.AEAD
	authSlots chan struct{}
}

func New(ctx context.Context, cfg Config) (*App, error) {
	key, err := base64.StdEncoding.DecodeString(cfg.MasterKey)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("APP_MASTER_KEY must be base64 of exactly 32 random bytes")
	}
	u, err := url.Parse(cfg.Origin)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("APP_ORIGIN must be an absolute HTTP(S) origin")
	}
	cfg.Origin = strings.TrimRight(cfg.Origin, "/")
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	poolCfg.MaxConns = 12
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}
	return &App{DB: pool, Cfg: cfg, cipher: aead, authSlots: make(chan struct{}, 2)}, nil
}
func (a *App) Close() { a.DB.Close() }
func (a *App) Migrate(ctx context.Context) error {
	conn, err := a.DB.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(73519001)"); err != nil {
		return err
	}
	defer func() { _, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock(73519001)") }()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, schema); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (a *App) StorageInit() error {
	for _, dir := range []string{"assets", "originals", "responses", "tmp"} {
		p := filepath.Join(a.Cfg.DataDir, dir)
		if err := os.MkdirAll(p, 0700); err != nil {
			return err
		}
		if os.Geteuid() == 0 {
			if err := os.Chown(p, 65532, 65532); err != nil {
				return err
			}
		}
	}
	if os.Geteuid() == 0 {
		return os.Chown(a.Cfg.DataDir, 65532, 65532)
	}
	return nil
}
func (a *App) encrypt(s string) (string, error) {
	nonce := make([]byte, a.cipher.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(a.cipher.Seal(nonce, nonce, []byte(s), nil)), nil
}
func (a *App) decrypt(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(b) < a.cipher.NonceSize() {
		return "", fmt.Errorf("invalid encrypted secret")
	}
	plain, err := a.cipher.Open(nil, b[:a.cipher.NonceSize()], b[a.cipher.NonceSize():], nil)
	return string(plain), err
}
func newID() string { return uuid.NewString() }
func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func marshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
func tenantID(u User) string {
	if u.TenantID == nil {
		return ""
	}
	return *u.TenantID
}

type contextKey int

const userKey contextKey = 1

func currentUser(r *http.Request) User { u, _ := r.Context().Value(userKey).(User); return u }

type endpoint func(http.ResponseWriter, *http.Request) error

func (a *App) handle(fn endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			status, code, message := 500, "INTERNAL_ERROR", "服务暂时无法处理请求"
			var ae *APIError
			if errors.As(err, &ae) {
				status, code, message = ae.Status, ae.Code, ae.Message
			} else if errors.Is(err, pgx.ErrNoRows) {
				status, code, message = 404, "NOT_FOUND", "记录不存在或无权访问"
			}
			if status >= 500 {
				slog.Error("request failed", "request_id", middleware.GetReqID(r.Context()), "path", r.URL.Path, "error", err.Error())
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil, "error": map[string]string{"code": code, "message": message}, "request_id": middleware.GetReqID(r.Context())})
		}
	}
}
func reply(w http.ResponseWriter, r *http.Request, status int, data any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(map[string]any{"data": data, "error": nil, "request_id": middleware.GetReqID(r.Context())})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return problem(400, "INVALID_INPUT", "请求内容格式不正确")
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return problem(400, "INVALID_INPUT", "请求只能包含一个 JSON 对象")
	}
	return nil
}
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
func (a *App) audit(ctx context.Context, u User, action, entity string, data any) {
	_, err := a.DB.Exec(ctx, "INSERT INTO audit_logs(id,user_id,action,entity_id,data) VALUES($1,$2,$3,$4,$5)", newID(), u.ID, action, entity, marshal(data))
	if err != nil {
		slog.Error("audit write failed", "action", action, "error", err)
	}
}
func (a *App) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("trend_session")
		if err != nil {
			a.handle(func(http.ResponseWriter, *http.Request) error { return problem(401, "UNAUTHENTICATED", "请先登录") })(w, r)
			return
		}
		var u User
		err = a.DB.QueryRow(r.Context(), `SELECT u.id,u.tenant_id,u.email,u.name,u.role FROM tokens t JOIN users u ON u.id=t.user_id WHERE t.hash=$1 AND t.kind='session' AND t.expires_at>now() AND t.used_at IS NULL`, hash(cookie.Value)).Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &u.Role)
		if err != nil {
			a.handle(func(http.ResponseWriter, *http.Request) error {
				return problem(401, "UNAUTHENTICATED", "登录已过期，请重新登录")
			})(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}
func requireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := currentUser(r)
			if u.Role != role || (role == "merchant" && u.TenantID == nil) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(403)
				_ = json.NewEncoder(w).Encode(map[string]any{"data": nil, "error": map[string]string{"code": "FORBIDDEN", "message": "没有此操作权限"}, "request_id": middleware.GetReqID(r.Context())})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
func (a *App) originCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			if origin := r.Header.Get("Origin"); origin != "" && origin != a.Cfg.Origin {
				a.handle(func(http.ResponseWriter, *http.Request) error {
					return problem(403, "ORIGIN_DENIED", "请求来源不匹配")
				})(w, r)
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
func (a *App) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, a.originCheck)
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) { _ = reply(w, r, 200, map[string]string{"status": "ok"}) })
	r.Get("/health/ready", a.handle(func(w http.ResponseWriter, r *http.Request) error {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := a.DB.Ping(ctx); err != nil {
			return problem(503, "NOT_READY", "数据库不可用")
		}
		f, err := os.CreateTemp(filepath.Join(a.Cfg.DataDir, "tmp"), "ready-*")
		if err != nil {
			return problem(503, "NOT_READY", "素材存储不可写")
		}
		_ = f.Close()
		_ = os.Remove(f.Name())
		return reply(w, r, 200, map[string]string{"status": "ready"})
	}))
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", a.handle(a.login))
		r.Post("/auth/invite", a.handle(a.acceptInvite))
		r.Post("/auth/reset", a.handle(a.resetPassword))
		r.Group(func(r chi.Router) {
			r.Use(a.authenticate)
			r.Get("/auth/me", a.handle(a.me))
			r.Post("/auth/logout", a.handle(a.logout))
			r.Group(func(r chi.Router) {
				r.Use(requireRole("merchant"))
				r.Get("/presets", a.handle(a.presets))
				r.Get("/products", a.handle(a.listProducts))
				r.Post("/products", a.handle(a.saveProduct))
				r.Get("/products/{id}", a.handle(a.getProductHTTP))
				r.Put("/products/{id}", a.handle(a.saveProduct))
				r.Delete("/products/{id}", a.handle(a.archiveProduct))
				r.Post("/products/{id}/references", a.handle(a.uploadReference))
				r.Put("/products/{id}/references", a.handle(a.orderReferences))
				r.Delete("/products/{id}/references/{asset}", a.handle(a.removeReference))
				r.Post("/products/{id}/analyze", a.handle(a.analyzeProduct))
				r.Get("/topics", a.handle(a.topicsHTTP))
				r.Post("/topic-imports", a.handle(a.importTopicHTTP))
				r.Get("/topic-imports/{id}", a.handle(a.getImportHTTP))
				r.Post("/generations/quote", a.handle(a.quoteHTTP))
				r.Post("/generations", a.handle(a.submitHTTP))
				r.Get("/generations", a.handle(a.listGenerations))
				r.Get("/generations/{id}", a.handle(a.generationHTTP))
				r.Post("/generations/{id}/retry", a.handle(a.retryQuoteHTTP))
				r.Post("/generations/{id}/copy", a.handle(a.saveCopyHTTP))
				r.Post("/generations/{id}/cancel", a.handle(a.cancelHTTP))
				r.Post("/generations/{id}/selection", a.handle(a.selectAssetHTTP))
				r.Post("/generations/{id}/export", a.handle(a.exportHTTP))
				r.Get("/assets/{id}", a.handle(a.assetHTTP))
				r.Get("/credits", a.handle(a.creditsHTTP))
				r.Get("/credits/ledger", a.handle(a.ledgerHTTP))
			})
			r.Route("/admin", func(r chi.Router) { r.Use(requireRole("admin")); a.adminRoutes(r) })
		})
		r.NotFound(a.handle(func(http.ResponseWriter, *http.Request) error { return problem(404, "NOT_FOUND", "接口不存在") }))
	})
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")
		if strings.HasPrefix(rel, "api/") {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(a.Cfg.WebDir, rel)
		if s, err := os.Stat(path); err != nil || s.IsDir() {
			path = filepath.Join(a.Cfg.WebDir, "index.html")
		}
		http.ServeFile(w, r, path)
	})
	return r
}
