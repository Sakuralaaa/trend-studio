package studio

import (
	"context"
	"github.com/jackc/pgx/v5"
	"net/http"
)

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// The caller holds a transaction. Lock order is always tenant then ledger.
func (a *App) creditChange(ctx context.Context, db DBTX, tid, gid, iid, key, kind string, available, reserved int64, description string) error {
	var balance, hold int64
	if err := db.QueryRow(ctx, "SELECT available,reserved FROM tenants WHERE id=$1 FOR UPDATE", tid).Scan(&balance, &hold); err != nil {
		return err
	}
	var previousTenant, previousKind, previousGeneration, previousItem string
	var previousAvailable, previousReserved int64
	err := db.QueryRow(ctx, "SELECT tenant_id,type,COALESCE(generation_id::text,''),COALESCE(item_id::text,''),available_delta,reserved_delta FROM credit_ledger WHERE event_key=$1", key).Scan(&previousTenant, &previousKind, &previousGeneration, &previousItem, &previousAvailable, &previousReserved)
	if err == nil {
		if previousTenant != tid || previousKind != kind || previousGeneration != gid || previousItem != iid || previousAvailable != available || previousReserved != reserved {
			return problem(409, "KEY_CONFLICT", "额度操作标识已用于另一项调整")
		}
		return nil
	}
	if err != pgx.ErrNoRows {
		return err
	}
	if balance+available < 0 || hold+reserved < 0 {
		return problem(409, "INSUFFICIENT_CREDITS", "可用额度不足")
	}
	if _, err := db.Exec(ctx, "UPDATE tenants SET available=available+$1,reserved=reserved+$2 WHERE id=$3", available, reserved, tid); err != nil {
		return err
	}
	_, err = db.Exec(ctx, "INSERT INTO credit_ledger(id,tenant_id,generation_id,item_id,event_key,type,available_delta,reserved_delta,balance_after,reserved_after,description) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)", newID(), tid, nullable(gid), nullable(iid), key, kind, available, reserved, balance+available, hold+reserved, description)
	return err
}
func (a *App) creditsHTTP(w http.ResponseWriter, r *http.Request) error {
	t, err := a.tenant(r.Context(), tenantID(currentUser(r)))
	if err != nil {
		return err
	}
	return reply(w, r, 200, t)
}
func (a *App) ledgerHTTP(w http.ResponseWriter, r *http.Request) error {
	limit, offset := page(r)
	rows, err := a.DB.Query(r.Context(), "SELECT id,type,available_delta,reserved_delta,balance_after,reserved_after,description,created_at FROM credit_ledger WHERE tenant_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3", tenantID(currentUser(r)), limit, offset)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		v, err := rows.Values()
		if err != nil {
			return err
		}
		m := map[string]any{}
		for i, k := range []string{"id", "type", "available_delta", "reserved_delta", "balance_after", "reserved_after", "description", "created_at"} {
			m[k] = v[i]
		}
		items = append(items, m)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return reply(w, r, 200, map[string]any{"items": items})
}

var _ = pgx.ErrNoRows
