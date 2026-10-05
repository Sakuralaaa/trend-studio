// External-service fixture for CI only. Not copied into the production image.
package main
import("bytes";"encoding/base64";"encoding/json";"image";"image/color";"image/draw";"image/png";"net/http";"strings";"sync";"time";"fmt";"os")
var mu sync.Mutex
var calls=map[string]int{}
func main(){
 http.HandleFunc("/health",func(w http.ResponseWriter,r *http.Request){fmt.Fprint(w,"ok")})
 http.HandleFunc("/stats",func(w http.ResponseWriter,r *http.Request){mu.Lock();defer mu.Unlock();json.NewEncoder(w).Encode(calls)})
 http.HandleFunc("/v1/models",func(w http.ResponseWriter,r *http.Request){json.NewEncoder(w).Encode(map[string]any{"data":[]any{map[string]string{"id":"fixture"}}})})
 http.HandleFunc("/v1/chat/completions",text);http.HandleFunc("/v1/images/edits",edit)
 http.HandleFunc("/api/v1/parse",func(w http.ResponseWriter,r *http.Request){w.WriteHeader(202);json.NewEncoder(w).Encode(map[string]any{"data":map[string]string{"task_id":"fixture-import"}})})
 http.HandleFunc("/api/v1/tasks/",func(w http.ResponseWriter,r *http.Request){json.NewEncoder(w).Encode(map[string]any{"data":map[string]any{"state":"done","data":post()}})})
 http.HandleFunc("/api/v1/douyin/user/posts",func(w http.ResponseWriter,r *http.Request){json.NewEncoder(w).Encode(map[string]any{"data":map[string]any{"items":[]any{post()},"cursor":0,"has_more":false}})})
 addr:=os.Getenv("LISTEN_ADDR");if addr==""{addr=":8091"};if err:=http.ListenAndServe(addr,nil);err!=nil{panic(err)}
}
func text(w http.ResponseWriter,r *http.Request){
 var in struct{Messages []struct{Content json.RawMessage}};json.NewDecoder(r.Body).Decode(&in)
 var facts struct{Product struct{Name string};Preset string;Candidates []struct{ID string}};if len(in.Messages)>1{var content string;json.Unmarshal(in.Messages[len(in.Messages)-1].Content,&content);json.Unmarshal([]byte(content),&facts)}
 mu.Lock();calls["text:"+facts.Product.Name]++;mu.Unlock();var content any
 if len(facts.Candidates)>0{matches:=[]map[string]any{};for _,c:=range facts.Candidates{matches=append(matches,map[string]any{"id":c.ID,"score":85,"reason":"商品使用场景相符","hook":"日常搭配灵感","angle":"完整展示商品"})};content=map[string]any{"matches":matches}}else{
 titles:=[]string{facts.Product.Name+"，让日常更从容"};if facts.Preset=="douyin_sales"{titles=[]string{facts.Product.Name+"日常分享","一件商品的三种表达","看看我的日常搭配"}}
 content=map[string]any{"titles":titles,"description":"以真实参考图展现商品，适合日常使用与清晰商品展示。","selling_points":[]string{"完整展示商品","适合日常使用"}}
 if facts.Preset=="douyin_sales"{content.(map[string]any)["script"]=map[string]any{"text":"今天分享这件商品。先看看外观，再展示日常使用场景。选择适合自己的款式，了解详情后再决定。","estimated_seconds":30};content.(map[string]any)["caption"]=map[string]any{"text":"分享我的日常商品使用灵感","tags":[]string{"日常分享"}}}
 };b,_:=json.Marshal(content);json.NewEncoder(w).Encode(map[string]any{"choices":[]any{map[string]any{"message":map[string]string{"content":string(b)}}},"usage":map[string]int{"prompt_tokens":100,"completion_tokens":150}})
}
func edit(w http.ResponseWriter,r *http.Request){
 if err:=r.ParseMultipartForm(12<<20);err!=nil{http.Error(w,"multipart required",400);return};defer r.MultipartForm.RemoveAll()
 prompt:=r.FormValue("prompt");main:=strings.Contains(prompt,"简洁干净电商主图");kind:="scene";if main{kind="main"};mu.Lock();calls["image:"+kind]++;mu.Unlock()
 if strings.Contains(prompt,"[PARTIAL]")&&main{http.Error(w,"fixture rejects main image",422);return};if strings.Contains(prompt,"[UNKNOWN]")&&main{time.Sleep(12*time.Second)}
 var original image.Image;for _,headers:=range r.MultipartForm.File{if len(headers)>0{f,_:=headers[0].Open();original,_,_=image.Decode(f);f.Close();break}}
 if original==nil{http.Error(w,"reference required",422);return};canvas:=image.NewRGBA(image.Rect(0,0,600,600));draw.Draw(canvas,canvas.Bounds(),image.NewUniform(color.RGBA{244,241,234,255}),image.Point{},draw.Src)
 draw.Draw(canvas,image.Rect(50,50,550,550),original,original.Bounds().Min,draw.Over);var b bytes.Buffer;png.Encode(&b,canvas);json.NewEncoder(w).Encode(map[string]any{"data":[]any{map[string]string{"b64_json":base64.StdEncoding.EncodeToString(b.Bytes())}}})
}
func post()map[string]any{return map[string]any{"content_id":"fixture-fashion-1","web_url":"https://www.douyin.com/video/1234567890","title":"秋日自然色系的日常穿搭","description":"从温柔的日常色调寻找商品展示灵感","tags":[]string{"日常","穿搭"},"created_at":time.Now().Add(-2*time.Hour).UTC(),"fetched_at":time.Now().UTC(),"author":map[string]string{"nickname":"行业示例账号"},"stats":map[string]int{"digg_count":1200,"comment_count":80,"share_count":30,"collect_count":100}}}
