import {useState} from 'react'
import {Photo} from './Photo'
export function ImageCompare({original,generated}:{original:string;generated:string}){
 const [slider,setSlider]=useState(false),[position,setPosition]=useState(50)
 return <div><label className="check"><input type="checkbox" checked={slider} onChange={e=>setSlider(e.target.checked)}/>相同构图，使用滑块对比</label>{slider?<div className="comparison-slider"><Photo src={generated} alt="生成图" contain/><div style={{clipPath:'inset(0 '+(100-position)+'% 0 0)'}}><Photo src={original} alt="参考原图" contain/></div><input aria-label="原图对比位置" type="range" value={position} onChange={e=>setPosition(+e.target.value)}/></div>:<div className="comparison"><figure><Photo src={original} alt="参考原图" contain/><figcaption>参考原图</figcaption></figure><figure><Photo src={generated} alt="生成图" contain/><figcaption>生成图</figcaption></figure></div>}<a href={generated} target="_blank" rel="noreferrer">放大查看颜色、印花和标签</a></div>
}
