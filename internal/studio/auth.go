package studio

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/argon2"
)

func passwordHash(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	key := argon2.IDKey([]byte(password), salt, 2, 64*1024, 2, 32)
	return "argon2id$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key)
}
func passwordMatches(encoded, password string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 3 || p[0] != "argon2id" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(p[1])
	if err != nil || len(salt) != 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(p[2])
	if err != nil || len(expected) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 2, 64*1024, 2, 32)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
func validatePassword(s string) error {
	if len(s) < 8 || len(s) > 128 {
		return problem(400, "INVALID_PASSWORD", "密码长度需为8至128字节")
	}
	return nil
}
func validEmail(s string) bool {
	return strings.Contains(s, "@") && len(s) <= 254 && !strings.ContainsAny(s, "\r\n ")
}
func (a *App) Bootstrap(ctx context.Context, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) {
		return fmt.Errorf("valid BOOTSTRAP_ADMIN_EMAIL required")
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(73519002)"); err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE role='admin')").Scan(&exists); err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("administrator already exists; use password reset")
	}
	_, err = tx.Exec(ctx, "INSERT INTO users(id,email,name,password_hash,role) VALUES($1,$2,$3,$4,'admin')", newID(), email, "平台管理员", passwordHash(password))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (a *App) setSession(ctx context.Context, w http.ResponseWriter, id string) error {
	token := newToken()
	expires := time.Now().Add(7 * 24 * time.Hour)
	_, err := a.DB.Exec(ctx, "INSERT INTO tokens(hash,kind,user_id,expires_at) VALUES($1,'session',$2,$3)", hash(token), id, expires)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: "trend_session", Value: token, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.Cfg.Origin, "https://"), SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: 7 * 86400})
	return nil
}
func (a *App) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if len(in.Password) > 128 {
		return problem(400, "INVALID_INPUT", "密码过长")
	}
	select {
	case a.authSlots <- struct{}{}:
		defer func() { <-a.authSlots }()
	default:
		return problem(429, "RATE_LIMITED", "登录请求较多，请稍后再试")
	}
	// DB-backed rate window; no user-controlled forwarded address is trusted.
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	throttle := "login:" + hash(host+strings.ToLower(strings.TrimSpace(in.Email)))
	var n int
	err := a.DB.QueryRow(r.Context(), `INSERT INTO settings(key,data) VALUES($1,jsonb_build_object('n',1,'until',extract(epoch FROM now()+interval '5 minutes')))
 ON CONFLICT(key) DO UPDATE SET data=CASE WHEN (settings.data->>'until')::numeric<extract(epoch FROM now()) THEN EXCLUDED.data ELSE jsonb_set(settings.data,'{n}',to_jsonb((settings.data->>'n')::int+1)) END
 RETURNING (data->>'n')::int`, throttle).Scan(&n)
	if err != nil {
		return err
	}
	if n > 20 {
		return problem(429, "RATE_LIMITED", "登录尝试过多，请稍后再试")
	}
	var u User
	var ph string
	err = a.DB.QueryRow(r.Context(), "SELECT id,tenant_id,email,name,role,password_hash FROM users WHERE email=$1", strings.ToLower(strings.TrimSpace(in.Email))).Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &u.Role, &ph)
	if err != nil || !passwordMatches(ph, in.Password) {
		return problem(401, "INVALID_CREDENTIALS", "邮箱或密码不正确")
	}
	if err = a.setSession(r.Context(), w, u.ID); err != nil {
		return err
	}
	return reply(w, r, 200, u)
}
func (a *App) me(w http.ResponseWriter, r *http.Request) error {
	u := currentUser(r)
	var tenant *Tenant
	if u.TenantID != nil {
		t, err := a.tenant(r.Context(), *u.TenantID)
		if err != nil {
			return err
		}
		tenant = &t
	}
	vision := false
	if p, err := a.provider(r.Context(), "text", ""); err == nil {
		vision = p.VisionModel != ""
	}
	return reply(w, r, 200, map[string]any{"user": u, "tenant": tenant, "capabilities": map[string]bool{"vision": vision}})
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie("trend_session"); err == nil {
		if _, err = a.DB.Exec(r.Context(), "DELETE FROM tokens WHERE hash=$1", hash(c.Value)); err != nil {
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "trend_session", Value: "", Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.Cfg.Origin, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	return reply(w, r, 200, map[string]bool{"success": true})
}

type InviteData struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Credits int64  `json:"default_credits"`
}

func (a *App) acceptInvite(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Token    string `json:"token"`
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if err := validatePassword(in.Password); err != nil {
		return err
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !validEmail(in.Email) || len([]rune(in.Name)) < 1 || len([]rune(in.Name)) > 80 {
		return problem(400, "INVALID_INPUT", "请填写有效邮箱和店铺名称")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var raw []byte
	err = tx.QueryRow(r.Context(), "SELECT data FROM tokens WHERE hash=$1 AND kind='invite' AND expires_at>now() AND used_at IS NULL FOR UPDATE", hash(in.Token)).Scan(&raw)
	if err != nil {
		return problem(400, "INVALID_INVITE", "邀请已失效或已使用")
	}
	var invite InviteData
	if err = jsonUnmarshal(raw, &invite); err != nil {
		return err
	}
	if invite.Email != "" && invite.Email != in.Email {
		return problem(400, "INVITE_EMAIL_MISMATCH", "邮箱与邀请对象不一致")
	}
	var exists bool
	if err = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)", in.Email).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return problem(409, "EMAIL_EXISTS", "此邮箱已注册")
	}
	tid, uid := newID(), newID()
	if _, err = tx.Exec(r.Context(), "INSERT INTO tenants(id,name) VALUES($1,$2)", tid, in.Name); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), "INSERT INTO users(id,tenant_id,email,name,password_hash,role) VALUES($1,$2,$3,$4,$5,'merchant')", uid, tid, in.Email, in.Name, passwordHash(in.Password)); err != nil {
		return err
	}
	if err = a.creditChange(r.Context(), tx, tid, "", "", "invite:"+hash(in.Token), "grant", invite.Credits, 0, "邀请内测额度"); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), "UPDATE tokens SET used_at=now() WHERE hash=$1", hash(in.Token)); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	if err = a.setSession(r.Context(), w, uid); err != nil {
		return err
	}
	return reply(w, r, 201, map[string]string{"id": uid})
}
func (a *App) resetPassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if err := validatePassword(in.Password); err != nil {
		return err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var uid string
	if err = tx.QueryRow(r.Context(), "SELECT user_id FROM tokens WHERE hash=$1 AND kind='reset' AND expires_at>now() AND used_at IS NULL FOR UPDATE", hash(in.Token)).Scan(&uid); err != nil {
		return problem(400, "INVALID_RESET", "重置链接已失效")
	}
	if _, err = tx.Exec(r.Context(), "UPDATE users SET password_hash=$1 WHERE id=$2", passwordHash(in.Password), uid); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), "DELETE FROM tokens WHERE user_id=$1 AND kind='session'", uid); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), "UPDATE tokens SET used_at=now() WHERE hash=$1", hash(in.Token)); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	return reply(w, r, 200, map[string]bool{"success": true})
}
func (a *App) tenant(ctx context.Context, id string) (Tenant, error) {
	var t Tenant
	err := a.DB.QueryRow(ctx, "SELECT id,name,available,reserved FROM tenants WHERE id=$1", id).Scan(&t.ID, &t.Name, &t.Balance, &t.Reserved)
	return t, err
}

// pgx errors are mapped at the HTTP boundary; malformed JSON is never silently accepted.
var _ = pgx.ErrNoRows
