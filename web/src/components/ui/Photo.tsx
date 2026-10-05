import {useState} from 'react'
import {ImageOff} from 'lucide-react'
export function Photo({src,alt,contain=false}:{src?:string;alt:string;contain?:boolean}){
 const [failedFor,setFailed]=useState<string>()
 return src&&failedFor!==src?<img src={src} alt={alt} loading="lazy" className={contain?'contain':''} onError={()=>setFailed(src)}/>:<div className="photo-fallback"><ImageOff size={24}/><span>{src?'图片未加载':'暂无参考图'}</span>{src&&<button onClick={()=>setFailed(undefined)}>重试</button>}</div>
}
