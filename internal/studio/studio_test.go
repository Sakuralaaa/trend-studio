package studio
import("archive/zip";"bytes";"context";"encoding/base64";"encoding/json";"image";"io";"mime/multipart";"net/http";"net/http/httptest";"net/url";"os";"path/filepath";"strings";"sync";"testing";"time";"github.com/jackc/pgx/v5";"github.com/go-chi/chi/v5")
func TestGrowth(t *testing.T){v:=func(n int64)*int64{return &n};now:=Counters{v(20),v(4),v(3),v(2)};old:=Counters{v(10),v(2),v(1),v(1)};if interaction(Counters{})!=nil{t.Fatal("missing counters cannot be zero")};if growth(now,old,.25)!=nil{t.Fatal("minimum interval")};if got:=growth(now,old,1);got==nil||*got!=22{t.Fatalf("weighted growth: %v",got)};if growth(old,now,1)!=nil{t.Fatal("counter rollback")}}
func TestPackages(t *testing.T){for _,tc:=range []struct{preset string;images int;credits int64}{{Showcase,2,23},{Douyin,1,13}}{var cost int64;images:=0;for _,i:=range planItems(tc.preset,Pricing{Text:3,Image:10}){cost+=i.Credits;if i.ProviderType=="image"{images++}};if cost!=tc.credits||images!=tc.images{t.Fatal(tc)}}}
func TestSSRF(t *testing.T){for _,address:=range []string{"http://127.0.0.1/a","http://[::1]/a","file:///tmp/a","http://169.254.169.254/"}{if validatedURL(address,false)==nil{t.Fatal(address)}}}
func TestPassword(t *testing.T){p:=passwordHash("StrongPassword9");if !passwordMatches(p,"StrongPassword9")||passwordMatches(p,"other"){t.Fatal("argon2 verification")}}
func TestDatabaseWorkflow(t *testing.T){
 raw:=os.Getenv("TEST_DATABASE_URL");if raw==""{t.Skip("PostgreSQL integration runs in Actions")};ctx:=context.Background()
 root,err:=pgx.Connect(ctx,raw);if err!=nil{t.Fatal(err)};defer root.Close(ctx);schemaName:="test_"+strings.ReplaceAll(newID(),"-","")
 if _,err=root.Exec(ctx,"CREATE SCHEMA "+schemaName);err!=nil{t.Fatal(err)};defer root.Exec(ctx,"DROP SCHEMA "+schemaName+" CASCADE")
 u,_:=url.Parse(raw);q:=u.Query();q.Set("search_path",schemaName);u.RawQuery=q.Encode()
 app,err:=New(ctx,Config{DatabaseURL:u.String(),MasterKey:base64.StdEncoding.EncodeToString(make([]byte,32)),Origin:"http://localhost",DataDir:t.TempDir(),FontPath:os.Getenv("FONT_PATH")});if err!=nil{t.Fatal(err)};defer app.Close()
 if err=app.StorageInit();err!=nil{t.Fatal(err)};if err=app.Migrate(ctx);err!=nil{t.Fatal(err)}
 tid,uid:=newID(),newID();_,err=app.DB.Exec(ctx,"INSERT INTO tenants(id,name,available) VALUES($1,'test',100);",tid);if err!=nil{t.Fatal(err)}
 if _,err=app.DB.Exec(ctx,"INSERT INTO users(id,tenant_id,email,name,password_hash,role) VALUES($1,$2,'test@example.com','test','x','merchant')",uid,tid);err!=nil{t.Fatal(err)}
 user:=User{ID:uid,TenantID:&tid,Role:"merchant"}
 call:=func(method,path string,body any)*httptest.ResponseRecorder{r:=httptest.NewRequest(method,path,bytes.NewReader(marshal(body)));r=r.WithContext(context.WithValue(r.Context(),userKey,user));w:=httptest.NewRecorder();switch path{case "/quote":app.handle(app.quoteHTTP)(w,r);case "/submit":app.handle(app.submitHTTP)(w,r)};return w}
 for _,kind:=range []string{"image","text"}{p:=Provider{Kind:kind,Model:"fixture",BaseURL:"http://fixture/v1",MaxReferences:6,ImageField:"image[]",Size:"1024x1024",TimeoutSeconds:30};encrypted,_:=app.encrypt("secret");if _,err=app.DB.Exec(ctx,"INSERT INTO provider_configs(id,kind,data,encrypted_key) VALUES($1,$2,$3,$4)",newID(),kind,marshal(p),encrypted);err!=nil{t.Fatal(err)}}
 ids:=[]string{newID(),newID()};for i,id:=range ids{p:=Product{Name:[]string{"first","second"}[i],Category:"家居日用",Points:[]string{},Scenarios:[]string{}};if _,err=app.DB.Exec(ctx,"INSERT INTO products(id,tenant_id,data) VALUES($1,$2,$3)",id,tid,marshal(p));err!=nil{t.Fatal(err)}
 aid:=newID();path:=filepath.Join(app.Cfg.DataDir,"originals",aid+".png"); img:=image.NewRGBA(image.Rect(0,0,10,10));b,_:=encodePNG(img);if err=atomicFile(path,b);err!=nil{t.Fatal(err)}
 if _,err=app.DB.Exec(ctx,"INSERT INTO assets(id,tenant_id,product_id,kind,path,meta) VALUES($1,$2,$3,'reference',$4,$5)",aid,tid,id,path,marshal(AssetMeta{Width:10,Height:10}));err!=nil{t.Fatal(err)}}
 w:=call("POST","/quote",map[string]string{"product_id":ids[1]});if w.Code!=200{t.Fatal(w.Body.String())};var envelope struct{Data Quote};if err=json.Unmarshal(w.Body.Bytes(),&envelope);err!=nil{t.Fatal(err)};quote:=envelope.Data;if quote.Preset!=Showcase||quote.Total!=23{t.Fatal(quote)}
 var wg sync.WaitGroup;responses:=make(chan *httptest.ResponseRecorder,12);for n:=0;n<12;n++{wg.Add(1);go func(){defer wg.Done();responses<-call("POST","/submit",map[string]string{"quote_id":quote.ID,"idempotency_key":quote.IdempotencyKey})}()};wg.Wait();close(responses)
 for response:=range responses{if response.Code!=200&&response.Code!=201{t.Fatal(response.Body.String())}}
 var gid,pid string;var count int;if err=app.DB.QueryRow(ctx,"SELECT count(*) FROM generations").Scan(&count);err!=nil||count!=1{t.Fatal("duplicate generation",count,err)}
 if err=app.DB.QueryRow(ctx,"SELECT id,product_id FROM generations").Scan(&gid,&pid);err!=nil||pid!=ids[1]{t.Fatal("wrong product",pid,err)}
 tenant,err:=app.tenant(ctx,tid);if err!=nil||tenant.Balance!=77||tenant.Reserved!=23{t.Fatal("reserve",tenant,err)}
 other:=newID();_,_=app.DB.Exec(ctx,"INSERT INTO tenants(id,name) VALUES($1,'other')",other);if _,err=app.product(ctx,app.DB,other,pid);err!=pgx.ErrNoRows{t.Fatal("product tenant leak",err)};if _,err=app.generation(ctx,app.DB,other,gid);err!=pgx.ErrNoRows{t.Fatal("generation tenant leak",err)}
 g,err:=app.generation(ctx,app.DB,tid,gid);if err!=nil{t.Fatal(err)};for _,item:=range g.Items{if item.ProviderType=="template"{continue};tx,_:=app.DB.Begin(ctx);if err=app.creditChange(ctx,tx,tid,gid,item.ID,"settle:test:"+item.ID,"settle",0,-item.Credits,"test");err!=nil{t.Fatal(err)};if err=app.creditChange(ctx,tx,tid,gid,item.ID,"settle:test:"+item.ID,"settle",0,-item.Credits,"duplicate");err!=nil{t.Fatal(err)};if err=tx.Commit(ctx);err!=nil{t.Fatal(err)}}
 tenant,_=app.tenant(ctx,tid);if tenant.Reserved!=0||tenant.Balance!=77{t.Fatal("double settlement",tenant)}
 // Quote must expire even when a client ignores its clock.
 _,_=app.DB.Exec(ctx,"UPDATE quotes SET generation_id=NULL,expires_at=now()-interval '1 second' WHERE id=$1",quote.ID)
 expired:=call("POST","/submit",map[string]string{"quote_id":quote.ID,"idempotency_key":newID()});if expired.Code!=409||!strings.Contains(expired.Body.String(),"QUOTE_EXPIRED"){t.Fatal(expired.Body.String())}
 // Real export writes image bytes and the current copy version.
 iid:=g.Items[0].ID;b,_:=encodePNG(image.NewRGBA(image.Rect(0,0,5,5)));aid,err:=app.addAsset(ctx,tid,gid,iid,"main_image",b,AssetMeta{Width:5,Height:5});if err!=nil{t.Fatal(err)}
 _,_=app.DB.Exec(ctx,"UPDATE generation_items SET selected_asset_id=$1 WHERE id=$2",aid,iid)
 _,_=app.DB.Exec(ctx,"UPDATE generations SET copy_version=2,copy_data=$1 WHERE id=$2",marshal(CopyData{Titles:[]string{"saved"},Description:"latest",Points:[]string{}}),gid)
 r:=httptest.NewRequest("POST","/export",nil);rc:=chiContext("id",gid);r=r.WithContext(context.WithValue(context.WithValue(r.Context(),userKey,user),chi.RouteCtxKey,rc));out:=httptest.NewRecorder();if err=app.exportHTTP(out,r);err!=nil{t.Fatal(err)}
 archive,err:=zip.NewReader(bytes.NewReader(out.Body.Bytes()),int64(out.Body.Len()));if err!=nil{t.Fatal(err)};foundPNG,foundCopy:=false,false;for _,file:=range archive.File{f,_:=file.Open();content,_:=io.ReadAll(f);f.Close();if strings.HasSuffix(file.Name,".png"){foundPNG=true;if !bytes.HasPrefix(content,[]byte{137,80,78,71}){t.Fatal("export only contains links")}};if file.Name=="copy.json"{foundCopy=bytes.Contains(content,[]byte("latest"))}};if !foundPNG||!foundCopy{t.Fatal("incomplete export")}
 // Multipart validation rejects fake images.
 var body bytes.Buffer;mp:=multipart.NewWriter(&body);dest,_:=mp.CreateFormFile("file","fake.png");_,_=dest.Write([]byte("not an image"));_=mp.Close();r=httptest.NewRequest("POST","/upload",&body);r.Header.Set("Content-Type",mp.FormDataContentType());r=r.WithContext(context.WithValue(context.WithValue(r.Context(),userKey,user),chi.RouteCtxKey,chiContext("id",pid)));out=httptest.NewRecorder();app.handle(app.uploadReference)(out,r);if out.Code!=400{t.Fatal("invalid image accepted")}
 _=time.Now()
}
func chiContext(key,value string)*chi.Context{r:=chi.NewRouteContext();r.URLParams.Add(key,value);return r}
