package studio
import(
 "archive/zip";"context";"encoding/json";"fmt";"io";"net/http";"os";"time"
 "github.com/go-chi/chi/v5";"github.com/jackc/pgx/v5"
)
func(a *App) pricing(ctx context.Context,db DBTX)(Pricing,error){var p Pricing;var b []byte;err:=db.QueryRow(ctx,"SELECT data FROM settings WHERE key='pricing'").Scan(&b);if err==nil{err=json.Unmarshal(b,&p)};return p,err}
func(a *App) limits(ctx context.Context)(Limits,error){var p Limits;var b []byte;err:=a.DB.QueryRow(ctx,"SELECT data FROM settings WHERE key='limits'").Scan(&b);if err==nil{err=json.Unmarshal(b,&p)};return p,err}
func(a *App) presets(w http.ResponseWriter,r *http.Request)error{
 p,err:=a.pricing(r.Context(),a.DB);if err!=nil{return err};items:=[]map[string]any{}
 for _,preset:=range []string{Showcase,Douyin}{entries:=planItems(preset,p);var sum int64;for _,entry:=range entries{sum+=entry.Credits};name:="商品展示";if preset==Douyin{name="抖音带货"};items=append(items,map[string]any{"id":preset,"name":name,"items":entries,"total_credits":sum})}
 return reply(w,r,200,map[string]any{"default":Showcase,"presets":items,"pricing_version":p.Version})
}
func(a *App) quoteHTTP(w http.ResponseWriter,r *http.Request)error{
 var in struct{ProductID string `json:"product_id"`;Preset string `json:"preset"`;TopicID string `json:"topic_id"`}
 if err:=decode(w,r,&in);err!=nil{return err};if in.Preset==""{in.Preset=Showcase};if in.Preset!=Showcase&&in.Preset!=Douyin{return problem(400,"INVALID_PRESET","内容类型无效")}
 p,err:=a.product(r.Context(),a.DB,tenantID(currentUser(r)),in.ProductID);if err!=nil{return err};if p.Status!="active"||len(p.References)==0{return problem(400,"REFERENCE_REQUIRED","请先上传至少一张商品参考图")}
 text,err:=a.provider(r.Context(),"text","");if err!=nil{return problem(409,"TEXT_UNCONFIGURED","管理员尚未配置文字接口")}
 img,err:=a.provider(r.Context(),"image","");if err!=nil{return problem(409,"IMAGE_UNCONFIGURED","管理员尚未配置图片编辑接口")}
 if len(p.References)>img.MaxReferences{return problem(400,"REFERENCE_CAPABILITY","参考图数量超过当前图片接口能力，请联系管理员")}
 pricing,err:=a.pricing(r.Context(),a.DB);if err!=nil{return err}
 q:=Quote{ID:newID(),ProductID:p.ID,ProductName:p.Name,Preset:in.Preset,TopicID:in.TopicID,Items:planItems(in.Preset,pricing),ExpiresAt:time.Now().UTC().Add(10*time.Minute),IdempotencyKey:newID(),Snapshot:Snapshot{Product:p,TextProviderID:text.ID,ImageProviderID:img.ID,Pricing:pricing,PromptVersion:PromptVersion,TemplateVersion:TemplateVersion}}
 if in.TopicID!=""{topic,err:=a.topic(r.Context(),tenantID(currentUser(r)),in.TopicID);if err!=nil{return err};q.Snapshot.Topic=&topic}
 return a.storeQuote(w,r,q)
}
func(a *App) storeQuote(w http.ResponseWriter,r *http.Request,q Quote)error{
 for _,item:=range q.Items{q.Total+=item.Credits}
 _,err:=a.DB.Exec(r.Context(),"INSERT INTO quotes(id,tenant_id,data,expires_at) VALUES($1,$2,$3,$4)",q.ID,tenantID(currentUser(r)),marshal(StoredQuote{q,q.Snapshot}),q.ExpiresAt);if err!=nil{return err};return reply(w,r,200,q)
}
func(a *App) submitHTTP(w http.ResponseWriter,r *http.Request)error{
 var in struct{QuoteID string `json:"quote_id"`;Key string `json:"idempotency_key"`};if err:=decode(w,r,&in);err!=nil{return err};if len(in.Key)<8||len(in.Key)>100{return problem(400,"INVALID_KEY","任务标识无效")}
 ctx:=r.Context();tid:=tenantID(currentUser(r));tx,err:=a.DB.Begin(ctx);if err!=nil{return err};defer tx.Rollback(context.Background())
 var lockID string;if err=tx.QueryRow(ctx,"SELECT id FROM tenants WHERE id=$1 FOR UPDATE",tid).Scan(&lockID);err!=nil{return err}
 var gid,qid string;err=tx.QueryRow(ctx,"SELECT generation_id,quote_id FROM idempotency WHERE tenant_id=$1 AND key=$2",tid,in.Key).Scan(&gid,&qid)
 if err==nil{if qid!=in.QuoteID{return problem(409,"KEY_CONFLICT","此任务标识已用于其他生成")};return reply(w,r,200,map[string]string{"id":gid})};if err!=pgx.ErrNoRows{return err}
 var raw []byte;var used *string;var expiry time.Time
 if err=tx.QueryRow(ctx,"SELECT data,generation_id,expires_at FROM quotes WHERE id=$1 AND tenant_id=$2 FOR UPDATE",in.QuoteID,tid).Scan(&raw,&used,&expiry);err!=nil{return err}
 if used!=nil{return problem(409,"QUOTE_USED","本次生成已提交")};if time.Now().After(expiry){return problem(409,"QUOTE_EXPIRED","报价已过期，请重新获取")}
 var stored StoredQuote;if err=json.Unmarshal(raw,&stored);err!=nil{return err};q:=stored.Quote;q.Snapshot=stored.Snapshot
 var version int;var status string
 if err=tx.QueryRow(ctx,"SELECT version,status FROM products WHERE id=$1 AND tenant_id=$2 FOR UPDATE",q.ProductID,tid).Scan(&version,&status);err!=nil{return err}
 pricing,err:=a.pricing(ctx,tx);if err!=nil{return err};if version!=q.Snapshot.Product.Version||status!="active"||pricing.Version!=q.Snapshot.Pricing.Version{return problem(409,"QUOTE_STALE","商品或价格已更新，请重新获取报价")}
 gid=newID()
 if q.GenerationID!=""{
  gid=q.GenerationID;var state string;if err=tx.QueryRow(ctx,"SELECT status FROM generations WHERE id=$1 AND tenant_id=$2 FOR UPDATE",gid,tid).Scan(&state);err!=nil{return err}
  if state=="queued"||state=="running"||state=="cancelled"{return problem(409,"TASK_BUSY","任务正在执行或已取消")}
  for _,it:=range q.Items{var itemState string;if err=tx.QueryRow(ctx,"SELECT status FROM generation_items WHERE generation_id=$1 AND kind=$2 FOR UPDATE",gid,it.Kind).Scan(&itemState);err!=nil{return err};if itemState=="needs_review"{return problem(409,"NEEDS_REVIEW","费用结果待核实，请联系管理员")}}
 }else{if _,err=tx.Exec(ctx,"INSERT INTO generations(id,tenant_id,product_id,preset,snapshot) VALUES($1,$2,$3,$4,$5)",gid,tid,q.ProductID,q.Preset,marshal(q.Snapshot));err!=nil{return err}}
 if err=a.creditChange(ctx,tx,tid,gid,"","reserve:"+q.ID,"reserve",-q.Total,q.Total,"生成素材预占");err!=nil{return err}
 for _,it:=range q.Items{
  if _,err=tx.Exec(ctx,`INSERT INTO generation_items(id,generation_id,kind,credits,quote_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(generation_id,kind) DO UPDATE SET status='pending',credits=EXCLUDED.credits,quote_id=EXCLUDED.quote_id,error=NULL`,newID(),gid,it.Kind,it.Credits,q.ID);err!=nil{return err}
 }
 if _,err=tx.Exec(ctx,"UPDATE generations SET status='queued',updated_at=now() WHERE id=$1",gid);err!=nil{return err}
 if _,err=tx.Exec(ctx,"INSERT INTO idempotency(tenant_id,key,quote_id,generation_id) VALUES($1,$2,$3,$4)",tid,in.Key,q.ID,gid);err!=nil{return err}
 if _,err=tx.Exec(ctx,"UPDATE quotes SET generation_id=$1 WHERE id=$2",gid,q.ID);err!=nil{return err}
 if err=a.enqueue(ctx,tx,"generation",tid,gid,"generation:"+q.ID,nil);err!=nil{return err}
 if err=tx.Commit(ctx);err!=nil{return err};return reply(w,r,201,map[string]string{"id":gid})
}
func(a *App) generation(ctx context.Context,db DBTX,tid,id string)(Generation,error){
 var g Generation;var snap,copy []byte
 err:=db.QueryRow(ctx,"SELECT id,product_id,preset,status,snapshot,copy_data,copy_version,render_pending,created_at,updated_at FROM generations WHERE id=$1 AND tenant_id=$2",id,tid).Scan(&g.ID,&g.ProductID,&g.Preset,&g.Status,&snap,&copy,&g.CopyVersion,&g.RenderPending,&g.CreatedAt,&g.UpdatedAt);if err!=nil{return g,err}
 if err=json.Unmarshal(snap,&g.Snapshot);err!=nil{return g,err};if err=json.Unmarshal(copy,&g.Copy);err!=nil{return g,err}
 p:=g.Snapshot.Product;g.ProductName=p.Name;g.ProductCategory=p.Category;if len(p.References)>0{g.ProductImageURL=p.References[0].URL};if g.Snapshot.Topic!=nil{g.TopicTitle=g.Snapshot.Topic.Title}
 rows,err:=db.Query(ctx,"SELECT id,kind,status,credits,error,selected_asset_id FROM generation_items WHERE generation_id=$1 ORDER BY kind",id);if err!=nil{return g,err}
 g.Items=[]Item{};selected:=map[string]*string{}
 for rows.Next(){var it Item;var sel *string;if err=rows.Scan(&it.ID,&it.Kind,&it.Status,&it.Credits,&it.Error,&sel);err!=nil{rows.Close();return g,err};it.Name=nameFor(it.Kind);it.ProviderType=providerFor(it.Kind);it.Versions=[]Asset{};selected[it.ID]=sel;g.Items=append(g.Items,it)};err=rows.Err();rows.Close();if err!=nil{return g,err}
 for i:=range g.Items{it:=&g.Items[i];rows,err=db.Query(ctx,"SELECT id,meta,created_at FROM assets WHERE item_id=$1 ORDER BY created_at DESC",it.ID);if err!=nil{return g,err}
 for rows.Next(){var asset Asset;var raw []byte;if err=rows.Scan(&asset.ID,&raw,&asset.CreatedAt);err!=nil{rows.Close();return g,err};if err=json.Unmarshal(raw,&asset.AssetMeta);err!=nil{rows.Close();return g,err};asset.Kind=it.Kind;asset.URL=assetURL(asset.ID);asset.Selected=selected[it.ID]!=nil&&*selected[it.ID]==asset.ID;it.Versions=append(it.Versions,asset);it.Width=asset.Width;it.Height=asset.Height};err=rows.Err();rows.Close();if err!=nil{return g,err}}
 err=db.QueryRow(ctx,`SELECT COALESCE(sum(-reserved_delta) FILTER(WHERE type='settle'),0),COALESCE(sum(available_delta) FILTER(WHERE type='release'),0),COALESCE(sum(reserved_delta),0) FROM credit_ledger WHERE generation_id=$1`,id).Scan(&g.Settled,&g.Released,&g.Reserved)
 return g,err
}
func(a *App) generationHTTP(w http.ResponseWriter,r *http.Request)error{g,err:=a.generation(r.Context(),a.DB,tenantID(currentUser(r)),chi.URLParam(r,"id"));if err!=nil{return err};return reply(w,r,200,g)}
func(a *App) listGenerations(w http.ResponseWriter,r *http.Request)error{
 limit,offset:=page(r);tid:=tenantID(currentUser(r));status:=r.URL.Query().Get("status")
 rows,err:=a.DB.Query(r.Context(),"SELECT id FROM generations WHERE tenant_id=$1 AND ($2='' OR status=$2) ORDER BY created_at DESC LIMIT $3 OFFSET $4",tid,status,limit,offset);if err!=nil{return err};ids:=[]string{}
 for rows.Next(){var id string;if err=rows.Scan(&id);err!=nil{rows.Close();return err};ids=append(ids,id)};err=rows.Err();rows.Close();if err!=nil{return err};items:=[]Generation{}
 for _,id:=range ids{g,err:=a.generation(r.Context(),a.DB,tid,id);if err!=nil{return err};items=append(items,g)}
 return reply(w,r,200,map[string]any{"items":items,"has_more":len(items)==limit})
}
func(a *App) retryQuoteHTTP(w http.ResponseWriter,r *http.Request)error{
 var in struct{Kind string `json:"item_type"`};if err:=decode(w,r,&in);err!=nil{return err}
 g,err:=a.generation(r.Context(),a.DB,tenantID(currentUser(r)),chi.URLParam(r,"id"));if err!=nil{return err};if g.Status=="running"||g.Status=="queued"||g.Status=="cancelled"{return problem(409,"TASK_BUSY","请等当前任务完成")}
 valid:=false;for _,it:=range g.Items{if it.Kind==in.Kind{if it.Status=="needs_review"{return problem(409,"NEEDS_REVIEW","此项费用待核实，不能自动重试")};valid=true}};if !valid||providerFor(in.Kind)=="template"{return problem(400,"INVALID_ITEM","请选择图片或文案；排版免费更新")}
 p,err:=a.pricing(r.Context(),a.DB);if err!=nil{return err};snapshot:=g.Snapshot;snapshot.Pricing=p
 // Retry preserves original product facts but uses current price and explicit provider versions.
 current,err:=a.product(r.Context(),a.DB,tenantID(currentUser(r)),g.ProductID);if err!=nil{return err};snapshot.Product.Version=current.Version
 price:=p.Image;if in.Kind=="copywriting"{price=p.Text}
 q:=Quote{ID:newID(),ProductID:g.ProductID,ProductName:g.ProductName,Preset:g.Preset,Items:[]QuoteItem{{in.Kind,nameFor(in.Kind),price,providerFor(in.Kind)}},ExpiresAt:time.Now().Add(10*time.Minute),IdempotencyKey:newID(),Snapshot:snapshot,GenerationID:g.ID,RetryKind:in.Kind}
 return a.storeQuote(w,r,q)
}
func validateCopy(c CopyData)error{
 if len(c.Titles)<1||len(c.Titles)>3||len(c.Points)>8||len([]rune(c.Description))>4000{return problem(400,"INVALID_COPY","文案结构或长度不正确")}
 for _,s:=range append(c.Titles,c.Points...){if len([]rune(s))>200{return problem(400,"INVALID_COPY","标题或卖点最多200字")}}
 if c.Script!=nil&&len([]rune(c.Script.Text))>2000{return problem(400,"INVALID_COPY","口播脚本过长")};if c.Caption!=nil&&len([]rune(c.Caption.Text))>2000{return problem(400,"INVALID_COPY","发布文案过长")};return nil
}
func(a *App) saveCopyHTTP(w http.ResponseWriter,r *http.Request)error{
 var in struct{Copy CopyData `json:"copy_data"`;Version int `json:"expected_version"`};if err:=decode(w,r,&in);err!=nil{return err};if err:=validateCopy(in.Copy);err!=nil{return err}
 ctx:=r.Context();tid,id:=tenantID(currentUser(r)),chi.URLParam(r,"id");tx,err:=a.DB.Begin(ctx);if err!=nil{return err};defer tx.Rollback(context.Background())
 var version int;var status string;if err=tx.QueryRow(ctx,"SELECT copy_version,status FROM generations WHERE id=$1 AND tenant_id=$2 FOR UPDATE",id,tid).Scan(&version,&status);err!=nil{return err}
 if version!=in.Version{return problem(409,"COPY_CONFLICT","文案已更新，请先同步最新版本")};if status=="queued"||status=="running"{return problem(409,"TASK_BUSY","请等待生成完成后修改文案")}
 if _,err=tx.Exec(ctx,"UPDATE generations SET copy_data=$1,copy_version=copy_version+1,render_pending=true,updated_at=now() WHERE id=$2",marshal(in.Copy),id);err!=nil{return err}
 if err=a.enqueue(ctx,tx,"render",tid,id,fmt.Sprintf("render:%s:%d",id,version+1),nil);err!=nil{return err};if err=tx.Commit(ctx);err!=nil{return err};return reply(w,r,200,map[string]any{"copy_version":version+1,"render_pending":true})
}
func(a *App) cancelHTTP(w http.ResponseWriter,r *http.Request)error{
 tid,id:=tenantID(currentUser(r)),chi.URLParam(r,"id");tx,err:=a.DB.Begin(r.Context());if err!=nil{return err};defer tx.Rollback(context.Background())
 var tenant string;if err=tx.QueryRow(r.Context(),"SELECT id FROM tenants WHERE id=$1 FOR UPDATE",tid).Scan(&tenant);err!=nil{return err}
 var status string;if err=tx.QueryRow(r.Context(),"SELECT status FROM generations WHERE id=$1 AND tenant_id=$2 FOR UPDATE",id,tid).Scan(&status);err!=nil{return err};if status!="queued"{return problem(409,"CANNOT_CANCEL","仅等待中的任务可以取消")}
 rows,err:=tx.Query(r.Context(),"SELECT id,credits,quote_id FROM generation_items WHERE generation_id=$1 AND status='pending'",id);if err!=nil{return err}
 type pending struct{id,q string;c int64};items:=[]pending{};for rows.Next(){var it pending;if err=rows.Scan(&it.id,&it.c,&it.q);err!=nil{rows.Close();return err};items=append(items,it)};rows.Close()
 for _,it:=range items{if err=a.creditChange(r.Context(),tx,tid,id,it.id,"release:"+it.q+":"+it.id,"release",it.c,-it.c,"取消生成退还");err!=nil{return err}}
 if _,err=tx.Exec(r.Context(),"UPDATE generations SET status='cancelled',updated_at=now() WHERE id=$1",id);err!=nil{return err};if _,err=tx.Exec(r.Context(),"UPDATE generation_items SET status='cancelled' WHERE generation_id=$1 AND status='pending'",id);err!=nil{return err}
 if _,err=tx.Exec(r.Context(),"UPDATE jobs SET status='done' WHERE generation_id=$1 AND status='queued'",id);err!=nil{return err};if err=tx.Commit(r.Context());err!=nil{return err};return reply(w,r,200,map[string]bool{"success":true})
}
func(a *App) selectAssetHTTP(w http.ResponseWriter,r *http.Request)error{
 var in struct{AssetID string `json:"asset_id"`};if err:=decode(w,r,&in);err!=nil{return err}
 tag,err:=a.DB.Exec(r.Context(),`UPDATE generation_items i SET selected_asset_id=a.id FROM assets a WHERE a.id=$1 AND a.tenant_id=$2 AND a.generation_id=$3 AND a.item_id=i.id`,in.AssetID,tenantID(currentUser(r)),chi.URLParam(r,"id"));if err!=nil{return err};if tag.RowsAffected()!=1{return pgx.ErrNoRows};return reply(w,r,200,map[string]bool{"success":true})
}
func(a *App) exportHTTP(w http.ResponseWriter,r *http.Request)error{
 g,err:=a.generation(r.Context(),a.DB,tenantID(currentUser(r)),chi.URLParam(r,"id"));if err!=nil{return err};if g.RenderPending{return problem(409,"RENDER_PENDING","图片更新中，请完成后再下载")}
 type file struct{name,path string};files:=[]file{};versions:=[]map[string]any{}
 for _,it:=range g.Items{for _,v:=range it.Versions{if v.Selected{
  var path string;if err=a.DB.QueryRow(r.Context(),"SELECT path FROM assets WHERE id=$1 AND tenant_id=$2",v.ID,tenantID(currentUser(r))).Scan(&path);err!=nil{return err};if _,err=os.Stat(path);err!=nil{return problem(409,"ASSET_MISSING","素材文件缺失，请联系管理员")}
  files=append(files,file{it.Kind+".png",path});versions=append(versions,map[string]any{"asset_id":v.ID,"item_type":it.Kind,"copy_version":v.CopyVersion,"created_at":v.CreatedAt})
 }}}
 manifest:=map[string]any{"generation_id":g.ID,"product_id":g.ProductID,"product_version":g.Snapshot.Product.Version,"preset":g.Preset,"topic":g.Snapshot.Topic,"copy_version":g.CopyVersion,"assets":versions,"created_at":g.CreatedAt,"exported_at":time.Now().UTC(),"template_version":g.Snapshot.TemplateVersion}
 w.Header().Set("Content-Type","application/zip");w.Header().Set("Content-Disposition",`attachment; filename="trend-studio-`+g.ID+`.zip"`);w.WriteHeader(200);z:=zip.NewWriter(w)
 write:=func(name string,b []byte)error{f,err:=z.Create(name);if err!=nil{return err};_,err=f.Write(b);return err}
 if err=write("manifest.json",marshal(manifest));err!=nil{return err};if err=write("copy.json",marshal(g.Copy));err!=nil{return err}
 if err=write("copy.txt",[]byte(fmt.Sprintf("%s\n\n%s\n\n%s\n",g.ProductName,g.Copy.Description,string(marshal(g.Copy)))));err!=nil{return err}
 for _,file:=range files{f,err:=os.Open(file.path);if err!=nil{return err};dest,err:=z.Create(file.name);if err==nil{_,err=io.Copy(dest,f)};f.Close();if err!=nil{return err}}
 return z.Close()
}
