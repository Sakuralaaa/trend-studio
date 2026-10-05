package studio
import(
 "bytes";"context";"encoding/base64";"encoding/json";"fmt";"image";"io";"mime/multipart";"net";"net/http";"net/url";"os";"path/filepath";"strings";"time"
)
func(a *App) provider(ctx context.Context,kind,id string)(Provider,error){
 var p Provider;var b []byte;var encrypted string
 query:="SELECT id,data,encrypted_key FROM provider_configs WHERE kind=$1 AND active";args:=[]any{kind};if id!=""{query="SELECT id,data,encrypted_key FROM provider_configs WHERE kind=$1 AND id=$2";args=append(args,id)}
 var pid string;err:=a.DB.QueryRow(ctx,query,args...).Scan(&pid,&b,&encrypted);if err!=nil{return p,err};if err=json.Unmarshal(b,&p);err!=nil{return p,err};p.ID=pid;p.Kind=kind;p.Configured=true;p.APIKey,err=a.decrypt(encrypted);return p,err
}
func validatedURL(raw string,allowPrivate bool)error{
 u,err:=url.Parse(raw);if err!=nil||u.Hostname()==""||u.User!=nil||(u.Scheme!="https"&&u.Scheme!="http"){return fmt.Errorf("invalid HTTP(S) URL")}
 if allowPrivate{return nil};ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
 ips,err:=net.DefaultResolver.LookupIPAddr(ctx,u.Hostname());if err!=nil||len(ips)==0{return fmt.Errorf("cannot resolve host")}
 for _,ip:=range ips{if unsafeIP(ip.IP){return fmt.Errorf("private result address forbidden")}}
 return nil
}
func unsafeIP(ip net.IP)bool{return ip.IsPrivate()||ip.IsLoopback()||ip.IsUnspecified()||ip.IsLinkLocalUnicast()||ip.IsLinkLocalMulticast()||ip.IsMulticast()}
// Validate the address at dial time as well, preventing DNS rebinding.
func secureClient(timeout time.Duration,allowPrivate bool)*http.Client{
 transport:=&http.Transport{DialContext:func(ctx context.Context,network,address string)(net.Conn,error){
 host,port,err:=net.SplitHostPort(address);if err!=nil{return nil,err};ips,err:=net.DefaultResolver.LookupIPAddr(ctx,host);if err!=nil{return nil,err}
 for _,ip:=range ips{if !allowPrivate&&unsafeIP(ip.IP){return nil,fmt.Errorf("private address forbidden")}}
 for _,ip:=range ips{d:=net.Dialer{Timeout:10*time.Second};conn,err:=d.DialContext(ctx,network,net.JoinHostPort(ip.IP.String(),port));if err==nil{return conn,nil}}
 return nil,fmt.Errorf("connection failed")
 }}
 return &http.Client{Timeout:timeout,Transport:transport,CheckRedirect:func(req *http.Request,via []*http.Request)error{if len(via)>=5{return fmt.Errorf("too many redirects")};req.Header.Del("Authorization");req.Header.Del("X-API-Key");return validatedURL(req.URL.String(),allowPrivate)}}
}
func providerEndpoint(p Provider,suffix string)string{return strings.TrimRight(p.BaseURL,"/")+suffix}
type remoteError struct{ message string; rejected bool }
func(e *remoteError)Error()string{return e.message}
func(a *App) requestModel(ctx context.Context,p Provider,preset,kind string,snapshot Snapshot)([]byte,error){
 var body io.Reader;contentType:="application/json";endpoint:=providerEndpoint(p,"/chat/completions")
 facts:=map[string]any{"product":snapshot.Product,"topic":snapshot.Topic,"preset":preset,"prompt_version":snapshot.PromptVersion}
 if kind=="copywriting"{
  prompt:="仅依据商品的已确认事实创作中文电商文案，不编造材质、功效、销量、价格或品牌。热点仅用于创意，不复制原作者文字。不含平台品牌标记。返回JSON，字段 titles:string[]（展示包1条，抖音包3条）, description:string, selling_points:string[]；抖音包额外 script:{text:string,estimated_seconds:30},caption:{text:string,tags:string[]}。卖点优先使用已确认卖点，无卖点时仅描述可确认商品信息。"
  body=bytes.NewReader(marshal(map[string]any{"model":p.Model,"response_format":map[string]string{"type":"json_object"},"messages":[]map[string]string{{"role":"system","content":prompt},{"role":"user","content":string(marshal(facts))}}}))
 }else{
  var b bytes.Buffer;mp:=multipart.NewWriter(&b);endpoint=providerEndpoint(p,"/images/edits")
  prompt:="编辑参考商品照片，忠实保留颜色、形状、印花、标志、标签和包装；不得改变商品本身，不添加虚构品牌或文字。"
  if kind=="main_image"{prompt+="制作简洁干净电商主图，完整展示商品主体。"}else{prompt+="制作自然真实的商品使用场景，商品主体完整可见，场景符合已确认用途。"}
  prompt+="商品资料："+string(marshal(facts))
  for _,pair:=range [][2]string{{"model",p.Model},{"prompt",prompt},{"n","1"},{"size",p.Size}}{if err:=mp.WriteField(pair[0],pair[1]);err!=nil{return nil,err}}
  for _,ref:=range snapshot.Product.References{
   var path string;if err:=a.DB.QueryRow(ctx,"SELECT path FROM assets WHERE id=$1 AND tenant_id=$2 AND kind='reference'",ref.ID,snapshot.Product.TenantID).Scan(&path);err!=nil{return nil,err}
   f,err:=os.Open(path);if err!=nil{return nil,err};dest,err:=mp.CreateFormFile(p.ImageField,filepath.Base(path));if err==nil{_,err=io.Copy(dest,f)};f.Close();if err!=nil{return nil,err}
  }
  if err:=mp.Close();err!=nil{return nil,err};body=&b;contentType=mp.FormDataContentType()
 }
 req,err:=http.NewRequestWithContext(ctx,"POST",endpoint,body);if err!=nil{return nil,err};req.Header.Set("Content-Type",contentType);req.Header.Set("Authorization","Bearer "+p.APIKey)
 client:=secureClient(time.Duration(p.TimeoutSeconds)*time.Second,os.Getenv("ALLOW_PRIVATE_PROVIDER")=="true");resp,err:=client.Do(req);if err!=nil{return nil,&remoteError{"接口请求已发送，结果待核实",false}};defer resp.Body.Close()
 raw,err:=io.ReadAll(io.LimitReader(resp.Body,(48<<20)+1));if err!=nil||len(raw)>48<<20{return nil,&remoteError{"接口响应不完整，结果待核实",false}}
 if resp.StatusCode<200||resp.StatusCode>=300{
  rejected:=resp.StatusCode==400||resp.StatusCode==401||resp.StatusCode==403||resp.StatusCode==404||resp.StatusCode==413||resp.StatusCode==422||resp.StatusCode==429
  return nil,&remoteError{fmt.Sprintf("提供商返回HTTP %d",resp.StatusCode),rejected}
 }
 return raw,nil
}
func parseCopy(raw []byte,preset string)(CopyData,error){
 var envelope struct{Choices []struct{Message struct{Content string `json:"content"`} `json:"message"`} `json:"choices"`}
 if err:=json.Unmarshal(raw,&envelope);err!=nil||len(envelope.Choices)==0{return CopyData{},fmt.Errorf("missing text choices")}
 text:=strings.TrimSpace(envelope.Choices[0].Message.Content);text=strings.TrimPrefix(text,"```json");text=strings.TrimPrefix(text,"```");text=strings.TrimSuffix(text,"```")
 var copy CopyData;if err:=json.Unmarshal([]byte(text),&copy);err!=nil{return copy,err};if err:=validateCopy(copy);err!=nil{return copy,err}
 if preset==Douyin&&(len(copy.Titles)!=3||copy.Script==nil||copy.Caption==nil){return copy,fmt.Errorf("incomplete douyin copy")};return copy,nil
}
func(a *App) parseImage(ctx context.Context,raw []byte,p Provider)([]byte,image.Config,error){
 var envelope struct{Data []struct{Base64 string `json:"b64_json"`;URL string `json:"url"`} `json:"data"`};var cfg image.Config
 if err:=json.Unmarshal(raw,&envelope);err!=nil||len(envelope.Data)!=1{return nil,cfg,fmt.Errorf("expected exactly one image")}
 var b []byte;var err error
 if envelope.Data[0].Base64!=""{b,err=base64.StdEncoding.DecodeString(envelope.Data[0].Base64)}else{
  rawURL:=envelope.Data[0].URL;allow:=os.Getenv("ALLOW_PRIVATE_PROVIDER")=="true";if err=validatedURL(rawURL,allow);err!=nil{return nil,cfg,err}
  u,_:=url.Parse(rawURL);validHost:=false;for _,host:=range p.ResultHosts{if strings.EqualFold(host,u.Hostname()){validHost=true}}
  // Empty allowlist permits any public address; configured allowlist also applies to redirects.
  if len(p.ResultHosts)>0&&!validHost{return nil,cfg,fmt.Errorf("result host not allowed")}
  client:=secureClient(60*time.Second,allow);originalRedirect:=client.CheckRedirect;client.CheckRedirect=func(req *http.Request,via []*http.Request)error{
   if err:=originalRedirect(req,via);err!=nil{return err};if len(p.ResultHosts)>0{for _,h:=range p.ResultHosts{if strings.EqualFold(h,req.URL.Hostname()){return nil}};return fmt.Errorf("redirect host not allowed")};return nil
  }
  req,err:=http.NewRequestWithContext(ctx,"GET",rawURL,nil);if err!=nil{return nil,cfg,err};resp,err:=client.Do(req);if err!=nil{return nil,cfg,err};defer resp.Body.Close()
  if resp.StatusCode!=200{return nil,cfg,fmt.Errorf("result download HTTP %d",resp.StatusCode)};b,err=io.ReadAll(io.LimitReader(resp.Body,(32<<20)+1))
 }
 if err!=nil||len(b)>32<<20{return nil,cfg,fmt.Errorf("invalid image response")}
 cfg,_,err=image.DecodeConfig(bytes.NewReader(b));if err!=nil||cfg.Width<=0||cfg.Height<=0||int64(cfg.Width)*int64(cfg.Height)>40_000_000{return nil,cfg,fmt.Errorf("invalid image dimensions")}
 img,_,err:=image.Decode(bytes.NewReader(b));if err!=nil{return nil,cfg,err};png,err:=encodePNG(img);return png,cfg,err
}
func(a *App) vision(ctx context.Context,p Provider,product Product)(Product,error){
 var path string;if err:=a.DB.QueryRow(ctx,"SELECT path FROM assets WHERE id=$1 AND tenant_id=$2",product.References[0].ID,product.TenantID).Scan(&path);err!=nil{return Product{},err};b,err:=os.ReadFile(path);if err!=nil{return Product{},err}
 reqBody:=map[string]any{"model":p.VisionModel,"response_format":map[string]string{"type":"json_object"},"messages":[]any{map[string]any{"role":"user","content":[]any{map[string]string{"type":"text","text":"仅识别可见商品：返回JSON {name,category,color,selling_points:[],usage_scenarios:[]}，category必须是："+strings.Join(Categories,"、")+"。不要猜测材质、功效、品牌，不能确认时留空。"},map[string]any{"type":"image_url","image_url":map[string]string{"url":"data:"+http.DetectContentType(b)+";base64,"+base64.StdEncoding.EncodeToString(b)}}}}}}
 req,err:=http.NewRequestWithContext(ctx,"POST",providerEndpoint(p,"/chat/completions"),bytes.NewReader(marshal(reqBody)));if err!=nil{return Product{},err};req.Header.Set("Content-Type","application/json");req.Header.Set("Authorization","Bearer "+p.APIKey)
 resp,err:=secureClient(time.Duration(p.TimeoutSeconds)*time.Second,os.Getenv("ALLOW_PRIVATE_PROVIDER")=="true").Do(req);if err!=nil{return Product{},err};defer resp.Body.Close();if resp.StatusCode!=200{return Product{},problem(502,"VISION_FAILED","识别失败，请稍后重试")}
 raw,err:=io.ReadAll(io.LimitReader(resp.Body,1<<20));if err!=nil{return Product{},err}
 var envelope struct{Choices []struct{Message struct{Content string}}};if err=json.Unmarshal(raw,&envelope);err!=nil||len(envelope.Choices)==0{return Product{},fmt.Errorf("invalid vision response")}
 var suggestion Product;err=json.Unmarshal([]byte(envelope.Choices[0].Message.Content),&suggestion);return suggestion,err
}
