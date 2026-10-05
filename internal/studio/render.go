package studio
import(
 "bytes";"context";"fmt";"image";"image/color";"image/draw";"os";"path/filepath";"strings"
 "golang.org/x/image/font";"golang.org/x/image/font/opentype";"golang.org/x/image/math/fixed";xdraw "golang.org/x/image/draw"
)
func wrapText(text string,face font.Face,width int,maxLines int)[]string{
 lines:=[]string{};line:="";for _,r:=range []rune(strings.TrimSpace(text)){
  next:=line+string(r);if r=='\n'||font.MeasureString(face,next).Ceil()>width{
   if line!=""{lines=append(lines,line)};line="";if r!='\n'{line=string(r)};if len(lines)>=maxLines{last:=[]rune(lines[maxLines-1]);for len(last)>0&&font.MeasureString(face,string(last)+"…").Ceil()>width{last=last[:len(last)-1]};lines[maxLines-1]=string(last)+"…";return lines[:maxLines]}
  }else{line=next}
 };if line!=""&&len(lines)<maxLines{lines=append(lines,line)};return lines
}
func(a *App) renderPNG(g Generation,source image.Image)([]byte,AssetMeta,error){
 height:=1080;if g.Preset==Douyin{height=1920};canvas:=image.NewRGBA(image.Rect(0,0,1080,height));draw.Draw(canvas,canvas.Bounds(),&image.Uniform{color.RGBA{250,250,250,255}},image.Point{},draw.Src)
 data,err:=os.ReadFile(a.Cfg.FontPath);if err!=nil{return nil,AssetMeta{},err};f,err:=opentype.Parse(data);if err!=nil{return nil,AssetMeta{},err}
 face,err:=opentype.NewFace(f,&opentype.FaceOptions{Size:42,DPI:72,Hinting:font.HintingNone});if err!=nil{return nil,AssetMeta{},err};defer face.Close()
 small,err:=opentype.NewFace(f,&opentype.FaceOptions{Size:30,DPI:72,Hinting:font.HintingNone});if err!=nil{return nil,AssetMeta{},err};defer small.Close()
 d:=font.Drawer{Dst:canvas,Src:image.NewUniform(color.RGBA{25,25,25,255}),Face:face}
 title:=g.ProductName;if len(g.Copy.Titles)>0{title=g.Copy.Titles[0]}
 y:=65;for _,line:=range wrapText(title,face,980,2){d.Dot=fixed.P(50,y);d.DrawString(line);y+=58}
 if g.Snapshot.Product.Brand!=""{d.Face=small;d.Dot=fixed.P(50,y+5);d.DrawString(strings.Join(wrapText(g.Snapshot.Product.Brand,small,980,1),""));d.Face=face}
 box:=image.Rect(50,200,1030,740);if height==1920{box=image.Rect(50,210,1030,1450)}
 sb:=source.Bounds();scale:=float64(box.Dx())/float64(sb.Dx());if other:=float64(box.Dy())/float64(sb.Dy());other<scale{scale=other}
 width,h:=int(float64(sb.Dx())*scale),int(float64(sb.Dy())*scale);target:=image.Rect(box.Min.X+(box.Dx()-width)/2,box.Min.Y+(box.Dy()-h)/2,box.Min.X+(box.Dx()+width)/2,box.Min.Y+(box.Dy()+h)/2)
 xdraw.BiLinear.Scale(canvas,target,source,sb,draw.Over,nil)
 d.Face=small;y=815;if height==1920{y=1530};max:=4
 points:=g.Copy.Points;if len(points)==0{points=g.Snapshot.Product.Points}
 for i,p:=range points{if i>=max{break};for _,line:=range wrapText(p,small,980,1){d.Dot=fixed.P(50,y);d.DrawString(line);y+=52}}
 b,err:=encodePNG(canvas);return b,AssetMeta{FileName:"template.png",FileSize:int64(len(b)),Width:1080,Height:height,CopyVersion:g.CopyVersion,Label:"排版版本 "+TemplateVersion},err
}
func(a *App) addAsset(ctx context.Context,tid,gid,iid,kind string,b []byte,meta AssetMeta)(string,error){
 id:=newID();path:=filepath.Join(a.Cfg.DataDir,"assets",id+".png");if err:=atomicFile(path,b);err!=nil{return "",err}
 _,err:=a.DB.Exec(ctx,"INSERT INTO assets(id,tenant_id,generation_id,item_id,kind,path,meta) VALUES($1,$2,$3,$4,$5,$6,$7)",id,tid,gid,iid,kind,path,marshal(meta));return id,err
}
func(a *App) renderGeneration(ctx context.Context,tid,gid string)error{
 g,err:=a.generation(ctx,a.DB,tid,gid);if err!=nil{return err}
 kind,sourceKind:="selling_point_image","main_image";if g.Preset==Douyin{kind,sourceKind="vertical_cover","scene_image"}
 iid,source:="","";for _,it:=range g.Items{if it.Kind==kind{iid=it.ID};if it.Kind==sourceKind{for _,asset:=range it.Versions{if asset.Selected{source=asset.ID}}}}
 if iid==""{return nil};if source==""||g.CopyVersion==0{
  _,err=a.DB.Exec(ctx,"UPDATE generation_items SET status='failed',error='缺少可用图片或文案，成功素材仍可下载' WHERE id=$1 AND status IN ('pending','running')",iid);return err
 }
 var path string;if err=a.DB.QueryRow(ctx,"SELECT path FROM assets WHERE id=$1 AND tenant_id=$2",source,tid).Scan(&path);err!=nil{return err};b,err:=os.ReadFile(path);if err!=nil{return err};img,_,err:=image.Decode(bytes.NewReader(b));if err!=nil{return err}
 png,meta,err:=a.renderPNG(g,img);if err!=nil{return err};id,err:=a.addAsset(ctx,tid,gid,iid,kind,png,meta);if err!=nil{return err}
 tx,err:=a.DB.Begin(ctx);if err!=nil{return err};defer tx.Rollback(context.Background())
 var version int;if err=tx.QueryRow(ctx,"SELECT copy_version FROM generations WHERE id=$1 FOR UPDATE",gid).Scan(&version);err!=nil{return err};if version==g.CopyVersion{
  if _,err=tx.Exec(ctx,"UPDATE generation_items SET status='succeeded',selected_asset_id=$1,error=NULL WHERE id=$2",id,iid);err!=nil{return err}
  if _,err=tx.Exec(ctx,"UPDATE generations SET render_pending=false,updated_at=now() WHERE id=$1",gid);err!=nil{return err}
 }
 return tx.Commit(ctx)
}
var _ = fmt.Sprint
