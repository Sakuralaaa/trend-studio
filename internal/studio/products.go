package studio

import (
 "context"
 "encoding/json"
 "fmt"
 "image"
 _ "image/jpeg"
 "image/png"
 "io"
 "net/http"
 "os"
 "path/filepath"
 "strconv"
 "strings"

 "github.com/go-chi/chi/v5"
 "github.com/jackc/pgx/v5"
 _ "golang.org/x/image/webp"
)

func page(r *http.Request)(int,int){n,_:=strconv.Atoi(r.URL.Query().Get("page"));if n<1{n=1};return 24,(n-1)*24}
func (a *App) product(ctx context.Context,db DBTX,tenant,id string)(Product,error){
 var p Product;var raw []byte
 err:=db.QueryRow(ctx,"SELECT data,version,status,created_at,updated_at FROM products WHERE id=$1 AND tenant_id=$2",id,tenant).Scan(&raw,&p.Version,&p.Status,&p.CreatedAt,&p.UpdatedAt)
 if err!=nil{return p,err};version,status,created,updated:=p.Version,p.Status,p.CreatedAt,p.UpdatedAt
 if err=json.Unmarshal(raw,&p);err!=nil{return p,err};p.ID=id;p.TenantID=tenant;p.Version=version;p.Status=status;p.CreatedAt=created;p.UpdatedAt=updated;p.References=[]Reference{}
 rows,err:=db.Query(ctx,"SELECT id,meta,position FROM assets WHERE tenant_id=$1 AND product_id=$2 AND kind='reference' AND NOT removed ORDER BY position,created_at",tenant,id)
 if err!=nil{return p,err};defer rows.Close()
 for rows.Next(){var ref Reference;var m AssetMeta;var b []byte;if err=rows.Scan(&ref.ID,&b,&ref.Position);err!=nil{return p,err};if err=json.Unmarshal(b,&m);err!=nil{return p,err};ref.URL=assetURL(ref.ID);ref.FileName=m.FileName;ref.FileSize=m.FileSize;ref.Width=m.Width;ref.Height=m.Height;ref.Primary=len(p.References)==0;p.References=append(p.References,ref)}
 return p,rows.Err()
}
func (a *App) listProducts(w http.ResponseWriter,r *http.Request)error{
 limit,offset:=page(r);tenant:=tenantID(currentUser(r));q:=r.URL.Query().Get("q");cat:=r.URL.Query().Get("category")
 filter:=`tenant_id=$1 AND status='active' AND ($2='' OR data->>'name' ILIKE '%'||$2||'%') AND ($3='' OR data->>'category'=$3)`
 var total int;if err:=a.DB.QueryRow(r.Context(),"SELECT count(*) FROM products WHERE "+filter,tenant,q,cat).Scan(&total);err!=nil{return err}
 rows,err:=a.DB.Query(r.Context(),"SELECT id FROM products WHERE "+filter+" ORDER BY updated_at DESC LIMIT $4 OFFSET $5",tenant,q,cat,limit,offset);if err!=nil{return err}
 ids:=[]string{};for rows.Next(){var id string;if err=rows.Scan(&id);err!=nil{rows.Close();return err};ids=append(ids,id)};err=rows.Err();rows.Close();if err!=nil{return err}
 products:=[]Product{};for _,id:=range ids{p,err:=a.product(r.Context(),a.DB,tenant,id);if err!=nil{return err};products=append(products,p)}
 return reply(w,r,200,map[string]any{"items":products,"total":total,"page_size":limit,"categories":Categories})
}
func validateProduct(p Product)error{
 if strings.TrimSpace(p.Name)==""||len([]rune(p.Name))>200{return problem(400,"INVALID_NAME","商品名称需为1至200字")}
 valid:=false;for _,c:=range Categories{if c==p.Category{valid=true}};if !valid{return problem(400,"INVALID_CATEGORY","请选择商品类目")}
 if len(p.Points)>8||len(p.Scenarios)>8{return problem(400,"INVALID_POINTS","卖点和场景最多各8条")}
 for _,s:=range append(p.Points,p.Scenarios...){if len([]rune(s))>200{return problem(400,"INVALID_POINTS","每条卖点或场景最多200字")}}
 if len([]rune(p.Brand))>80||len(p.SKU)>100||len([]rune(p.Color))>80||p.Price!=nil&&(*p.Price<0||*p.Price>1e9){return problem(400,"INVALID_PRODUCT","可选商品资料超出允许范围")};return nil
}
func (a *App) saveProduct(w http.ResponseWriter,r *http.Request)error{
 var p Product;if err:=decode(w,r,&p);err!=nil{return err};if err:=validateProduct(p);err!=nil{return err}
 tenant:=tenantID(currentUser(r));id:=chi.URLParam(r,"id");p.ID="";p.TenantID="";p.References=nil
 if id==""{id=newID();_,err:=a.DB.Exec(r.Context(),"INSERT INTO products(id,tenant_id,data) VALUES($1,$2,$3)",id,tenant,marshal(p));if err!=nil{return err}}else{
  tag,err:=a.DB.Exec(r.Context(),"UPDATE products SET data=$1,version=version+1,updated_at=now() WHERE id=$2 AND tenant_id=$3 AND status='active'",marshal(p),id,tenant);if err!=nil{return err};if tag.RowsAffected()==0{return pgx.ErrNoRows}
 }
 out,err:=a.product(r.Context(),a.DB,tenant,id);if err!=nil{return err};return reply(w,r,200,out)
}
func (a *App) getProductHTTP(w http.ResponseWriter,r *http.Request)error{p,err:=a.product(r.Context(),a.DB,tenantID(currentUser(r)),chi.URLParam(r,"id"));if err!=nil{return err};return reply(w,r,200,p)}
func (a *App) archiveProduct(w http.ResponseWriter,r *http.Request)error{
 tag,err:=a.DB.Exec(r.Context(),"UPDATE products SET status='archived',version=version+1,updated_at=now() WHERE id=$1 AND tenant_id=$2",chi.URLParam(r,"id"),tenantID(currentUser(r)));if err!=nil{return err};if tag.RowsAffected()==0{return pgx.ErrNoRows};return reply(w,r,200,map[string]bool{"success":true})
}
func atomicFile(path string,b []byte)error{
 f,err:=os.CreateTemp(filepath.Dir(path),".writing-*");if err!=nil{return err};defer os.Remove(f.Name())
 if err=f.Chmod(0600);err!=nil{f.Close();return err};if _,err=f.Write(b);err!=nil{f.Close();return err};if err=f.Sync();err!=nil{f.Close();return err};if err=f.Close();err!=nil{return err};return os.Rename(f.Name(),path)
}
func (a *App) uploadReference(w http.ResponseWriter,r *http.Request)error{
 r.Body=http.MaxBytesReader(w,r.Body,(10<<20)+(1<<20));if err:=r.ParseMultipartForm(10<<20);err!=nil{return problem(400,"INVALID_FILE","每张图片最多10 MiB")};defer r.MultipartForm.RemoveAll()
 f,header,err:=r.FormFile("file");if err!=nil{return problem(400,"INVALID_FILE","请选择图片文件")};defer f.Close()
 b,err:=io.ReadAll(io.LimitReader(f,(10<<20)+1));if err!=nil{return err};if len(b)>10<<20{return problem(400,"FILE_TOO_LARGE","每张图片最多10 MiB")}
 cfg,format,err:=image.DecodeConfig(strings.NewReader(string(b)));if err!=nil||(format!="jpeg"&&format!="png"&&format!="webp"){return problem(400,"INVALID_IMAGE","仅支持有效的 JPEG、PNG、WebP 图片")}
 if cfg.Width<=0||cfg.Height<=0||int64(cfg.Width)*int64(cfg.Height)>40_000_000{return problem(400,"IMAGE_TOO_LARGE","图片最多4000万像素")}
 tx,err:=a.DB.Begin(r.Context());if err!=nil{return err};defer tx.Rollback(context.Background())
 id,tenant:=chi.URLParam(r,"id"),tenantID(currentUser(r));var exists string
 if err=tx.QueryRow(r.Context(),"SELECT id FROM products WHERE id=$1 AND tenant_id=$2 AND status='active' FOR UPDATE",id,tenant).Scan(&exists);err!=nil{return err}
 var count int;if err=tx.QueryRow(r.Context(),"SELECT count(*) FROM assets WHERE product_id=$1 AND kind='reference' AND NOT removed",id).Scan(&count);err!=nil{return err};if count>=6{return problem(400,"REFERENCE_LIMIT","每件商品最多6张图片")}
 aid:=newID();path:=filepath.Join(a.Cfg.DataDir,"originals",aid+"."+format);if err=atomicFile(path,b);err!=nil{return err}
 meta:=AssetMeta{FileName:filepath.Base(header.Filename),FileSize:int64(len(b)),Width:cfg.Width,Height:cfg.Height}
 if _,err=tx.Exec(r.Context(),"INSERT INTO assets(id,tenant_id,product_id,kind,path,meta,position) VALUES($1,$2,$3,'reference',$4,$5,$6)",aid,tenant,id,path,marshal(meta),count);err!=nil{return err}
 if _,err=tx.Exec(r.Context(),"UPDATE products SET version=version+1,updated_at=now() WHERE id=$1",id);err!=nil{return err};if err=tx.Commit(r.Context());err!=nil{return err}
 return reply(w,r,201,Reference{ID:aid,URL:assetURL(aid),FileName:meta.FileName,FileSize:meta.FileSize,Width:cfg.Width,Height:cfg.Height,Position:count,Primary:count==0})
}
func (a *App) orderReferences(w http.ResponseWriter,r *http.Request)error{
 var in struct{IDs []string `json:"ids"`};if err:=decode(w,r,&in);err!=nil{return err}
 tenant,id:=tenantID(currentUser(r)),chi.URLParam(r,"id");tx,err:=a.DB.Begin(r.Context());if err!=nil{return err};defer tx.Rollback(context.Background())
 var pid string;if err=tx.QueryRow(r.Context(),"SELECT id FROM products WHERE id=$1 AND tenant_id=$2 FOR UPDATE",id,tenant).Scan(&pid);err!=nil{return err}
 p,err:=a.product(r.Context(),tx,tenant,id);if err!=nil{return err};if len(in.IDs)!=len(p.References){return problem(400,"INVALID_ORDER","请提供所有图片的顺序")}
 seen:=map[string]bool{};for i,aid:=range in.IDs{if seen[aid]{return problem(400,"INVALID_ORDER","图片不能重复")};seen[aid]=true
 tag,err:=tx.Exec(r.Context(),"UPDATE assets SET position=$1 WHERE id=$2 AND product_id=$3 AND tenant_id=$4 AND kind='reference' AND NOT removed",i,aid,id,tenant);if err!=nil{return err};if tag.RowsAffected()!=1{return problem(400,"INVALID_ORDER","图片不属于此商品")}}
 if _,err=tx.Exec(r.Context(),"UPDATE products SET version=version+1,updated_at=now() WHERE id=$1",id);err!=nil{return err};if err=tx.Commit(r.Context());err!=nil{return err};return reply(w,r,200,map[string]bool{"success":true})
}
func (a *App) removeReference(w http.ResponseWriter,r *http.Request)error{
 tx,err:=a.DB.Begin(r.Context());if err!=nil{return err};defer tx.Rollback(context.Background());id,tenant:=chi.URLParam(r,"id"),tenantID(currentUser(r))
 var pid string;if err=tx.QueryRow(r.Context(),"SELECT id FROM products WHERE id=$1 AND tenant_id=$2 FOR UPDATE",id,tenant).Scan(&pid);err!=nil{return err}
 tag,err:=tx.Exec(r.Context(),"UPDATE assets SET removed=true WHERE id=$1 AND product_id=$2 AND tenant_id=$3 AND kind='reference'",chi.URLParam(r,"asset"),id,tenant);if err!=nil{return err};if tag.RowsAffected()!=1{return pgx.ErrNoRows}
 if _,err=tx.Exec(r.Context(),"UPDATE products SET version=version+1,updated_at=now() WHERE id=$1",id);err!=nil{return err};if err=tx.Commit(r.Context());err!=nil{return err};return reply(w,r,200,map[string]bool{"success":true})
}
func (a *App) assetHTTP(w http.ResponseWriter,r *http.Request)error{
 var path string;if err:=a.DB.QueryRow(r.Context(),"SELECT path FROM assets WHERE id=$1 AND tenant_id=$2",chi.URLParam(r,"id"),tenantID(currentUser(r))).Scan(&path);err!=nil{return err}
 w.Header().Set("Cache-Control","private, max-age=300");http.ServeFile(w,r,path);return nil
}
func encodePNG(img image.Image)([]byte,error){var b strings.Builder;err:=png.Encode(&b,img);return []byte(b.String()),err}
func (a *App) analyzeProduct(w http.ResponseWriter,r *http.Request)error{
 p,err:=a.product(r.Context(),a.DB,tenantID(currentUser(r)),chi.URLParam(r,"id"));if err!=nil{return err}
 provider,err:=a.provider(r.Context(),"text","");if err!=nil||provider.VisionModel==""{return problem(409,"VISION_UNCONFIGURED","未配置视觉识别模型")}
 if len(p.References)==0{return problem(400,"REFERENCE_REQUIRED","请先上传参考图")}
 result,err:=a.vision(r.Context(),provider,p);if err!=nil{return err};return reply(w,r,200,map[string]any{"suggestions":result,"confirmation_required":true})
}
var _ = fmt.Sprint
