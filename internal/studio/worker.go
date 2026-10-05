package studio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func (a *App) enqueue(ctx context.Context, db DBTX, kind, tid, gid, dedupe string, data any) error {
	if data == nil {
		data = map[string]any{}
	}
	_, err := db.Exec(ctx, "INSERT INTO jobs(id,kind,tenant_id,generation_id,dedupe_key,data) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(dedupe_key) DO NOTHING", newID(), kind, nullable(tid), nullable(gid), dedupe, marshal(data))
	return err
}
func (a *App) claim(ctx context.Context, owner string) (Job, error) {
	var j Job
	err := a.DB.QueryRow(ctx, `WITH candidate AS (SELECT id FROM jobs WHERE (status='queued' OR status='running' AND lease_until<now()) AND next_run_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1)
 UPDATE jobs SET status='running',owner=$1,lease_until=now()+interval '60 seconds',attempts=attempts+1,updated_at=now() FROM candidate c WHERE jobs.id=c.id
 RETURNING jobs.id,kind,tenant_id,generation_id,data,attempts`, owner).Scan(&j.ID, &j.Kind, &j.TenantID, &j.GenerationID, &j.Data, &j.Attempts)
	return j, err
}
func (a *App) acquire(ctx context.Context, resource, owner string, limit int) (int, error) {
	for slot := 0; slot < limit; slot++ {
		var n int
		err := a.DB.QueryRow(ctx, `INSERT INTO resource_leases(resource,slot,owner,lease_until) VALUES($1,$2,$3,now()+interval '60 seconds')
 ON CONFLICT(resource,slot) DO UPDATE SET owner=EXCLUDED.owner,lease_until=EXCLUDED.lease_until WHERE resource_leases.lease_until<now() RETURNING slot`, resource, slot, owner).Scan(&n)
		if err == nil {
			return n, nil
		}
		if err != pgx.ErrNoRows {
			return 0, err
		}
	}
	return 0, problem(429, "CAPACITY_BUSY", "生成队列繁忙")
}
func (a *App) release(resource, owner string, slot int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = a.DB.Exec(ctx, "DELETE FROM resource_leases WHERE resource=$1 AND owner=$2 AND slot=$3", resource, owner, slot)
}
func (a *App) waitLease(ctx context.Context, resource, owner string, limit int) (int, error) {
	for {
		slot, err := a.acquire(ctx, resource, owner, limit)
		if err == nil {
			return slot, nil
		}
		var ae *APIError
		if !errors.As(err, &ae) {
			return 0, err
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
func (a *App) Worker(ctx context.Context) error {
	owner := newID()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	var wg sync.WaitGroup
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				j, err := a.claim(ctx, owner)
				if err != nil {
					if err != pgx.ErrNoRows {
						slog.Error("claim job", "error", err)
					}
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Second):
					}
					continue
				}
				a.runJob(ctx, owner, j)
			}
		}()
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = a.DB.Exec(ctx, "INSERT INTO worker_heartbeats(id) VALUES($1) ON CONFLICT(id) DO UPDATE SET updated_at=now()", owner)
				if err := a.scheduleSources(ctx); err != nil {
					slog.Error("schedule sources", "error", err)
				}
			}
		}
	}()
	<-ctx.Done()
	wg.Wait()
	return nil
}
func (a *App) runJob(parent context.Context, owner string, j Job) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	done := make(chan struct{})
	defer close(done)
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				tag, err := a.DB.Exec(ctx, "UPDATE jobs SET lease_until=now()+interval '60 seconds' WHERE id=$1 AND owner=$2 AND status='running'", j.ID, owner)
				if err != nil || tag.RowsAffected() != 1 {
					cancel()
					return
				}
				if _, err = a.DB.Exec(ctx, "UPDATE resource_leases SET lease_until=now()+interval '60 seconds' WHERE owner=$1 AND lease_until>now()", j.ID); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	var err error
	switch j.Kind {
	case "generation", "render":
		if j.TenantID == nil || j.GenerationID == nil {
			err = fmt.Errorf("missing task context")
			break
		}
		slot, e := a.acquire(ctx, "tenant:"+*j.TenantID, j.ID, 1)
		if e != nil {
			err = e
			break
		}
		defer a.release("tenant:"+*j.TenantID, j.ID, slot)
		if j.Kind == "generation" {
			err = a.executeGeneration(ctx, j)
		} else {
			err = a.renderGeneration(ctx, *j.TenantID, *j.GenerationID)
			if err == nil {
				err = a.finalize(ctx, *j.TenantID, *j.GenerationID)
			}
		}
	case "collect", "import":
		err = a.collectJob(ctx, j)
	default:
		err = fmt.Errorf("unknown job kind")
	}
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	state, next := "done", time.Now()
	if err != nil {
		state, next = "queued", time.Now().Add(30*time.Second)
		if j.Attempts >= 8 {
			state = "failed"
		}
		var ae *APIError
		if errors.As(err, &ae) && ae.Code == "CAPACITY_BUSY" {
			state = "queued"
		}
		slog.Warn("job incomplete", "job", j.ID, "kind", j.Kind, "error", err)
	}
	_, _ = a.DB.Exec(finishCtx, "UPDATE jobs SET status=$1,next_run_at=$2,error=$3,lease_until=NULL,updated_at=now() WHERE id=$4 AND owner=$5", state, next, errorString(err), j.ID, owner)
	if state == "failed" && j.Kind == "render" && j.GenerationID != nil {
		_, _ = a.DB.Exec(finishCtx, "UPDATE generations SET render_pending=false WHERE id=$1", *j.GenerationID)
		_, _ = a.DB.Exec(finishCtx, "UPDATE generation_items SET status='failed',error='排版失败，请保存文案后重试' WHERE generation_id=$1 AND kind IN ('selling_point_image','vertical_cover')", *j.GenerationID)
	}
}
func errorString(err error) any {
	if err == nil {
		return nil
	}
	return err.Error()
}
func (a *App) executeGeneration(ctx context.Context, j Job) error {
	tid, gid := *j.TenantID, *j.GenerationID
	g, err := a.generation(ctx, a.DB, tid, gid)
	if err != nil {
		return err
	}
	if g.Status == "cancelled" {
		return nil
	}
	if _, err = a.DB.Exec(ctx, "UPDATE generations SET status='running',updated_at=now() WHERE id=$1 AND status!='cancelled'", gid); err != nil {
		return err
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(g.Items))
	for _, it := range g.Items {
		if providerFor(it.Kind) == "template" || it.Status == "succeeded" || it.Status == "failed" || it.Status == "needs_review" {
			continue
		}
		wg.Add(1)
		go func(it Item) {
			defer wg.Done()
			if err := a.executeItem(ctx, j, g, it); err != nil {
				errs <- err
			}
		}(it)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	if err = a.renderGeneration(ctx, tid, gid); err != nil {
		return err
	}
	return a.finalize(ctx, tid, gid)
}
func (a *App) executeItem(ctx context.Context, j Job, g Generation, it Item) error {
	kind := providerFor(it.Kind)
	pid := g.Snapshot.ImageProviderID
	if kind == "text" {
		pid = g.Snapshot.TextProviderID
	}
	p, err := a.provider(ctx, kind, pid)
	if err != nil {
		return err
	}
	limits, err := a.limits(ctx)
	if err != nil {
		return err
	}
	limit := limits.Image
	if kind == "text" {
		limit = limits.Text
	}
	slot, err := a.waitLease(ctx, "provider:"+kind, j.ID, limit)
	if err != nil {
		return err
	}
	defer a.release("provider:"+kind, j.ID, slot)
	var attempt, state string
	var path *string
	err = a.DB.QueryRow(ctx, "SELECT id,status,response_path FROM provider_attempts WHERE item_id=$1 AND started_at >= (SELECT created_at FROM quotes WHERE id=(SELECT quote_id FROM generation_items WHERE id=$1)) ORDER BY started_at DESC LIMIT 1", it.ID).Scan(&attempt, &state, &path)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if err == pgx.ErrNoRows {
		attempt = newID()
		state = "prepared"
		_, err = a.DB.Exec(ctx, "INSERT INTO provider_attempts(id,item_id,generation_id,provider_id,kind) VALUES($1,$2,$3,$4,$5)", attempt, it.ID, g.ID, pid, kind)
		if err != nil {
			return err
		}
	}
	responsePath := filepath.Join(a.Cfg.DataDir, "responses", attempt+".json")
	raw, fileErr := os.ReadFile(responsePath)
	if state == "submitted" || state == "unknown" {
		if fileErr != nil {
			return a.finishItem(ctx, *j.TenantID, g.ID, it, "needs_review", "接口请求已发送，结果待管理员核实", attempt)
		}
		state = "response_saved"
	}
	if state == "rejected" {
		return a.finishItem(ctx, *j.TenantID, g.ID, it, "failed", "提供商明确拒绝请求", attempt)
	}
	if state == "prepared" {
		if _, err = a.DB.Exec(ctx, "UPDATE generation_items SET status='running' WHERE id=$1", it.ID); err != nil {
			return err
		}
		if _, err = a.DB.Exec(ctx, "UPDATE provider_attempts SET status='submitted' WHERE id=$1", attempt); err != nil {
			return err
		}
		start := time.Now()
		raw, err = a.requestModel(ctx, p, g.Preset, it.Kind, g.Snapshot)
		finishCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err != nil {
			rejected := false
			var re *remoteError
			if errors.As(err, &re) {
				rejected = re.rejected
			}
			status, itemState := "unknown", "needs_review"
			if rejected {
				status, itemState = "rejected", "failed"
			}
			if _, e := a.DB.Exec(finishCtx, "UPDATE provider_attempts SET status=$1,error=$2,duration_ms=$3,finished_at=now() WHERE id=$4", status, err.Error(), time.Since(start).Milliseconds(), attempt); e != nil {
				return e
			}
			return a.finishItem(finishCtx, *j.TenantID, g.ID, it, itemState, err.Error(), attempt)
		}
		if err = atomicFile(responsePath, raw); err != nil {
			return err
		}
		if _, err = a.DB.Exec(finishCtx, "UPDATE provider_attempts SET status='response_saved',response_path=$1,duration_ms=$2 WHERE id=$3", responsePath, time.Since(start).Milliseconds(), attempt); err != nil {
			return err
		}
	} else if fileErr != nil {
		return fmt.Errorf("saved response missing: %w", fileErr)
	}
	if kind == "text" {
		copy, err := parseCopy(raw, g.Preset)
		if err != nil {
			return a.finishItem(ctx, *j.TenantID, g.ID, it, "needs_review", "已收到模型响应，但文案格式无法确认", attempt)
		}
		if _, err = a.DB.Exec(ctx, "UPDATE generations SET copy_data=$1,copy_version=copy_version+1,updated_at=now() WHERE id=$2", marshal(copy), g.ID); err != nil {
			return err
		}
	} else {
		b, cfg, err := a.parseImage(ctx, raw, p)
		if err != nil {
			return a.finishItem(ctx, *j.TenantID, g.ID, it, "needs_review", "已收到模型响应，但图片处理失败，待核实", attempt)
		}
		asset, err := a.addAsset(ctx, *j.TenantID, g.ID, it.ID, it.Kind, b, AssetMeta{FileName: it.Kind + ".png", FileSize: int64(len(b)), Width: cfg.Width, Height: cfg.Height, Label: "生成图片"})
		if err != nil {
			return err
		}
		if _, err = a.DB.Exec(ctx, "UPDATE generation_items SET selected_asset_id=$1 WHERE id=$2", asset, it.ID); err != nil {
			return err
		}
	}
	var usage struct {
		Usage json.RawMessage `json:"usage"`
	}
	_ = json.Unmarshal(raw, &usage)
	if _, err = a.DB.Exec(ctx, "UPDATE provider_attempts SET usage=$1 WHERE id=$2", nullableJSON(usage.Usage), attempt); err != nil {
		return err
	}
	return a.finishItem(ctx, *j.TenantID, g.ID, it, "succeeded", "", attempt)
}
func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
func (a *App) finishItem(ctx context.Context, tid, gid string, it Item, state, message, attempt string) error {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var tenant string
	if err = tx.QueryRow(ctx, "SELECT id FROM tenants WHERE id=$1 FOR UPDATE", tid).Scan(&tenant); err != nil {
		return err
	}
	var qid string
	var cost int64
	if err = tx.QueryRow(ctx, "SELECT quote_id,credits FROM generation_items WHERE id=$1 FOR UPDATE", it.ID).Scan(&qid, &cost); err != nil {
		return err
	}
	if state == "succeeded" {
		err = a.creditChange(ctx, tx, tid, gid, it.ID, "settle:"+qid+":"+it.ID, "settle", 0, -cost, it.Name+"完成")
	} else if state == "failed" {
		err = a.creditChange(ctx, tx, tid, gid, it.ID, "release:"+qid+":"+it.ID, "release", cost, -cost, it.Name+"失败退还")
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE generation_items SET status=$1,error=$2 WHERE id=$3", state, nullable(message), it.ID); err != nil {
		return err
	}
	attemptState := state
	if state == "needs_review" {
		attemptState = "unknown"
	}
	if _, err = tx.Exec(ctx, "UPDATE provider_attempts SET status=$1,finished_at=now() WHERE id=$2", attemptState, attempt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (a *App) finalize(ctx context.Context, tid, gid string) error {
	g, err := a.generation(ctx, a.DB, tid, gid)
	if err != nil {
		return err
	}
	if g.Status == "cancelled" {
		return nil
	}
	success, failed, unknown := 0, 0, 0
	for _, it := range g.Items {
		switch it.Status {
		case "succeeded":
			success++
		case "failed":
			failed++
		case "needs_review":
			unknown++
		default:
			return nil
		}
	}
	status := "succeeded"
	if unknown > 0 {
		status = "needs_review"
	} else if failed > 0 {
		status = "failed"
		if success > 0 {
			status = "partial"
		}
	}
	_, err = a.DB.Exec(ctx, "UPDATE generations SET status=$1,updated_at=now() WHERE id=$2 AND tenant_id=$3", status, gid, tid)
	return err
}
