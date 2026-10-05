package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func (a *App) adminRoutes(r chi.Router) {
	r.Get("/overview", a.handle(a.overviewHTTP))
	r.Get("/tenants", a.handle(a.tenantsHTTP))
	r.Post("/invites", a.handle(a.inviteHTTP))
	r.Post("/resets", a.handle(a.resetLinkHTTP))
	r.Post("/credits", a.handle(a.grantHTTP))
	r.Get("/providers", a.handle(a.providersHTTP))
	r.Post("/providers", a.handle(a.saveProviderHTTP))
	r.Post("/providers/{id}/probe", a.handle(a.probeHTTP))
	r.Get("/settings", a.handle(a.settingsHTTP))
	r.Put("/pricing", a.handle(a.pricingHTTP))
	r.Put("/limits", a.handle(a.limitsHTTP))
	r.Put("/collector", a.handle(a.collectorConfigHTTP))
	r.Get("/sources", a.handle(a.sourcesHTTP))
	r.Post("/sources", a.handle(a.saveSourceHTTP))
	r.Put("/sources/{id}", a.handle(a.saveSourceHTTP))
	r.Get("/tasks", a.handle(a.adminTasksHTTP))
	r.Post("/tasks/{id}/resolve", a.handle(a.resolveHTTP))
}
func (a *App) overviewHTTP(w http.ResponseWriter, r *http.Request) error {
	var tenants, tasks, review, workers int64
	var estimated *float64
	if err := a.DB.QueryRow(r.Context(), `SELECT (SELECT count(*) FROM tenants),(SELECT count(*) FROM generations),(SELECT count(*) FROM generation_items WHERE status='needs_review'),(SELECT count(*) FROM worker_heartbeats WHERE updated_at>now()-interval '45 seconds'),(SELECT sum(estimated_cost) FROM provider_attempts)`).Scan(&tenants, &tasks, &review, &workers, &estimated); err != nil {
		return err
	}
	return reply(w, r, 200, map[string]any{"merchants": tenants, "generations": tasks, "needs_review": review, "workers": workers, "estimated_cost": estimated, "actual_cost": nil})
}
func (a *App) tenantsHTTP(w http.ResponseWriter, r *http.Request) error {
	rows, err := a.DB.Query(r.Context(), "SELECT id,name,available,reserved FROM tenants ORDER BY created_at DESC LIMIT 100")
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []Tenant{}
	for rows.Next() {
		var t Tenant
		if err = rows.Scan(&t.ID, &t.Name, &t.Balance, &t.Reserved); err != nil {
			return err
		}
		items = append(items, t)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return reply(w, r, 200, map[string]any{"items": items})
}
func (a *App) inviteHTTP(w http.ResponseWriter, r *http.Request) error {
	var in InviteData
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if in.Email != "" && !validEmail(in.Email) {
		return problem(400, "INVALID_EMAIL", "邮箱不正确")
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Credits == 0 {
		in.Credits = 100
	}
	if in.Credits < 0 || in.Credits > 100000 {
		return problem(400, "INVALID_CREDITS", "赠送额度超出范围")
	}
	token := newToken()
	expires := time.Now().Add(7 * 24 * time.Hour)
	_, err := a.DB.Exec(r.Context(), "INSERT INTO tokens(hash,kind,data,expires_at) VALUES($1,'invite',$2,$3)", hash(token), marshal(in), expires)
	if err != nil {
		return err
	}
	a.audit(r.Context(), currentUser(r), "invite", "", map[string]any{"email": in.Email, "credits": in.Credits})
	return reply(w, r, 201, map[string]any{"url": a.Cfg.Origin + "/activate?token=" + token, "expires_at": expires})
}
func (a *App) resetLinkHTTP(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email string `json:"email"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	var uid string
	if err := a.DB.QueryRow(r.Context(), "SELECT id FROM users WHERE email=$1", strings.ToLower(strings.TrimSpace(in.Email))).Scan(&uid); err != nil {
		return err
	}
	token := newToken()
	expires := time.Now().Add(time.Hour)
	_, err := a.DB.Exec(r.Context(), "INSERT INTO tokens(hash,kind,user_id,expires_at) VALUES($1,'reset',$2,$3)", hash(token), uid, expires)
	if err != nil {
		return err
	}
	a.audit(r.Context(), currentUser(r), "password_reset", uid, nil)
	return reply(w, r, 201, map[string]any{"url": a.Cfg.Origin + "/reset?token=" + token, "expires_at": expires})
}
func (a *App) grantHTTP(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		TenantID string `json:"tenant_id"`
		Credits  int64  `json:"credits"`
		Key      string `json:"idempotency_key"`
		Reason   string `json:"reason"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if in.Credits == 0 || in.Credits > 100000 || in.Credits < -100000 || len(in.Key) < 8 || in.Reason == "" {
		return problem(400, "INVALID_CREDITS", "请填写有效额度、原因和操作标识")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = a.creditChange(r.Context(), tx, in.TenantID, "", "", "admin:"+in.Key, "grant", in.Credits, 0, in.Reason); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	a.audit(r.Context(), currentUser(r), "credit_grant", in.TenantID, in)
	return reply(w, r, 200, map[string]bool{"success": true})
}
func (a *App) providersHTTP(w http.ResponseWriter, r *http.Request) error {
	rows, err := a.DB.Query(r.Context(), "SELECT id,data FROM provider_configs WHERE active ORDER BY kind")
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []Provider{}
	for rows.Next() {
		var p Provider
		var b []byte
		var id string
		if err = rows.Scan(&id, &b); err != nil {
			return err
		}
		if err = json.Unmarshal(b, &p); err != nil {
			return err
		}
		p.ID = id
		p.APIKey = ""
		p.Configured = true
		items = append(items, p)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return reply(w, r, 200, map[string]any{"items": items, "key_mask": "••••••••"})
}
func (a *App) saveProviderHTTP(w http.ResponseWriter, r *http.Request) error {
	var p Provider
	if err := decode(w, r, &p); err != nil {
		return err
	}
	if (p.Kind != "text" && p.Kind != "image") || p.Model == "" || p.APIKey == "" {
		return problem(400, "INVALID_PROVIDER", "请填写类型、模型和密钥")
	}
	if err := validatedURL(p.BaseURL, os.Getenv("ALLOW_PRIVATE_PROVIDER") == "true"); err != nil {
		return problem(400, "INVALID_URL", "接口地址不正确或指向受限地址")
	}
	u, _ := url.Parse(p.BaseURL)
	if u.RawQuery != "" || u.Fragment != "" {
		return problem(400, "INVALID_URL", "接口地址不能包含查询参数")
	}
	if p.TimeoutSeconds == 0 {
		p.TimeoutSeconds = 180
	}
	if p.TimeoutSeconds < 10 || p.TimeoutSeconds > 600 {
		return problem(400, "INVALID_TIMEOUT", "超时需为10至600秒")
	}
	if p.Kind == "image" {
		if p.ImageField == "" {
			p.ImageField = "image[]"
		}
		if p.ImageField != "image[]" && p.ImageField != "image" {
			return problem(400, "INVALID_IMAGE_FIELD", "参考图字段仅支持image或image[]")
		}
		if p.MaxReferences < 1 || p.MaxReferences > 6 {
			return problem(400, "INVALID_CAPABILITY", "图片编辑接口必须支持1至6张参考图")
		}
		if p.ImageField == "image" && p.MaxReferences > 1 {
			return problem(400, "INVALID_CAPABILITY", "多张参考图需使用image[]字段")
		}
		if p.Size == "" {
			p.Size = "1024x1024"
		}
		if p.Size != "1024x1024" && p.Size != "1024x1536" && p.Size != "1536x1024" && p.Size != "auto" {
			return problem(400, "INVALID_SIZE", "请选择支持的图片尺寸")
		}
	}
	encrypted, err := a.encrypt(p.APIKey)
	if err != nil {
		return err
	}
	p.APIKey = ""
	p.ID = newID()
	p.Configured = true
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(73519003)"); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), "UPDATE provider_configs SET active=false WHERE kind=$1", p.Kind); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), "INSERT INTO provider_configs(id,kind,data,encrypted_key) VALUES($1,$2,$3,$4)", p.ID, p.Kind, marshal(p), encrypted); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	a.audit(r.Context(), currentUser(r), "provider_config", p.ID, p)
	return reply(w, r, 201, p)
}
func (a *App) probeHTTP(w http.ResponseWriter, r *http.Request) error {
	var kind string
	if err := a.DB.QueryRow(r.Context(), "SELECT kind FROM provider_configs WHERE id=$1", chi.URLParam(r, "id")).Scan(&kind); err != nil {
		return err
	}
	p, err := a.provider(r.Context(), kind, chi.URLParam(r, "id"))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(r.Context(), "GET", providerEndpoint(p, "/models"), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	start := time.Now()
	resp, err := secureClient(15*time.Second, os.Getenv("ALLOW_PRIVATE_PROVIDER") == "true").Do(req)
	if err != nil {
		return problem(502, "PROBE_FAILED", "接口无法连接")
	}
	defer resp.Body.Close()
	return reply(w, r, 200, map[string]any{"http_status": resp.StatusCode, "latency_ms": time.Since(start).Milliseconds(), "generation_verified": false})
}
func (a *App) collectorConfig(ctx context.Context) (CollectorConfig, error) {
	var p CollectorConfig
	var b []byte
	var encrypted string
	err := a.DB.QueryRow(ctx, "SELECT data,encrypted_key FROM provider_configs WHERE kind='collector' AND active").Scan(&b, &encrypted)
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(b, &p); err != nil {
		return p, err
	}
	p.APIKey, err = a.decrypt(encrypted)
	p.Configured = err == nil
	return p, err
}
func (a *App) settingsHTTP(w http.ResponseWriter, r *http.Request) error {
	p, err := a.pricing(r.Context(), a.DB)
	if err != nil {
		return err
	}
	limits, err := a.limits(r.Context())
	if err != nil {
		return err
	}
	collector, _ := a.collectorConfig(r.Context())
	collector.APIKey = ""
	return reply(w, r, 200, map[string]any{"pricing": p, "limits": limits, "collector": collector})
}
func (a *App) pricingHTTP(w http.ResponseWriter, r *http.Request) error {
	var p Pricing
	if err := decode(w, r, &p); err != nil {
		return err
	}
	if p.Text < 0 || p.Image < 0 || p.Text > 10000 || p.Image > 10000 {
		return problem(400, "INVALID_PRICING", "点数超出范围")
	}
	_, err := a.DB.Exec(r.Context(), `UPDATE settings SET data=jsonb_build_object('text',$1::bigint,'image',$2::bigint,'version',(data->>'version')::int+1),updated_at=now() WHERE key='pricing'`, p.Text, p.Image)
	if err != nil {
		return err
	}
	a.audit(r.Context(), currentUser(r), "pricing", "", p)
	return a.settingsHTTP(w, r)
}
func (a *App) limitsHTTP(w http.ResponseWriter, r *http.Request) error {
	var p Limits
	if err := decode(w, r, &p); err != nil {
		return err
	}
	if p.Image < 1 || p.Image > 16 || p.Text < 1 || p.Text > 32 || p.Collect < 1 || p.Collect > 8 || p.Background < 0 || p.Background > 10000 {
		return problem(400, "INVALID_LIMITS", "并发或每日后台调用额度超出范围")
	}
	_, err := a.DB.Exec(r.Context(), "UPDATE settings SET data=$1,updated_at=now() WHERE key='limits'", marshal(p))
	if err != nil {
		return err
	}
	a.audit(r.Context(), currentUser(r), "limits", "", p)
	return a.settingsHTTP(w, r)
}
func (a *App) collectorConfigHTTP(w http.ResponseWriter, r *http.Request) error {
	var p CollectorConfig
	if err := decode(w, r, &p); err != nil {
		return err
	}
	if p.APIKey == "" {
		return problem(400, "INVALID_KEY", "请填写采集API Key")
	}
	if err := validatedURL(p.BaseURL, true); err != nil {
		return problem(400, "INVALID_URL", "采集服务地址无效")
	}
	encrypted, err := a.encrypt(p.APIKey)
	if err != nil {
		return err
	}
	p.APIKey = ""
	p.Configured = true
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(73519003)"); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), "UPDATE provider_configs SET active=false WHERE kind='collector'"); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), "INSERT INTO provider_configs(id,kind,data,encrypted_key) VALUES($1,'collector',$2,$3)", newID(), marshal(p), encrypted); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	return reply(w, r, 200, p)
}
func (a *App) adminTasksHTTP(w http.ResponseWriter, r *http.Request) error {
	rows, err := a.DB.Query(r.Context(), "SELECT id,tenant_id FROM generations WHERE status IN ('needs_review','failed','partial','running') ORDER BY updated_at DESC LIMIT 100")
	if err != nil {
		return err
	}
	pairs := [][2]string{}
	for rows.Next() {
		var p [2]string
		if err = rows.Scan(&p[0], &p[1]); err != nil {
			rows.Close()
			return err
		}
		pairs = append(pairs, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	items := []Generation{}
	for _, p := range pairs {
		g, err := a.generation(r.Context(), a.DB, p[1], p[0])
		if err != nil {
			return err
		}
		items = append(items, g)
	}
	return reply(w, r, 200, map[string]any{"items": items})
}
func (a *App) resolveHTTP(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		ItemID   string          `json:"item_id"`
		Decision string          `json:"decision"`
		Reason   string          `json:"reason"`
		Response json.RawMessage `json:"provider_response"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if len([]rune(in.Reason)) < 5 {
		return problem(400, "REASON_REQUIRED", "请记录供应商核实依据")
	}
	gid := chi.URLParam(r, "id")
	var tid, attempt string
	var it Item
	it.ID = in.ItemID
	if err := a.DB.QueryRow(r.Context(), `SELECT g.tenant_id,i.kind,i.credits FROM generation_items i JOIN generations g ON g.id=i.generation_id WHERE i.id=$1 AND g.id=$2 AND i.status='needs_review'`, it.ID, gid).Scan(&tid, &it.Kind, &it.Credits); err != nil {
		return err
	}
	it.Name = nameFor(it.Kind)
	if err := a.DB.QueryRow(r.Context(), "SELECT id FROM provider_attempts WHERE item_id=$1 ORDER BY started_at DESC LIMIT 1", it.ID).Scan(&attempt); err != nil {
		return err
	}
	if in.Decision == "release" {
		if err := a.finishItem(r.Context(), tid, gid, it, "failed", in.Reason, attempt); err != nil {
			return err
		}
	} else if in.Decision == "response" && len(in.Response) > 0 {
		path := a.Cfg.DataDir + "/responses/" + attempt + ".json"
		if err := atomicFile(path, in.Response); err != nil {
			return err
		}
		if _, err := a.DB.Exec(r.Context(), "UPDATE provider_attempts SET status='response_saved',response_path=$1 WHERE id=$2", path, attempt); err != nil {
			return err
		}
		if _, err := a.DB.Exec(r.Context(), "UPDATE generation_items SET status='pending',error=NULL WHERE id=$1", it.ID); err != nil {
			return err
		}
		if err := a.enqueue(r.Context(), a.DB, "generation", tid, gid, "resolve:"+newID(), nil); err != nil {
			return err
		}
	} else {
		return problem(400, "INVALID_DECISION", "请选择明确失败退还，或提供真实供应商响应")
	}
	a.audit(r.Context(), currentUser(r), "resolve", in.ItemID, map[string]string{"decision": in.Decision, "reason": in.Reason})
	if err := a.finalize(r.Context(), tid, gid); err != nil {
		return err
	}
	return reply(w, r, 200, map[string]bool{"success": true})
}

var _ = fmt.Sprint
