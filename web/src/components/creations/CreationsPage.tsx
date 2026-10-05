import {useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {Link,Navigate,useOutletContext} from 'react-router-dom'
import {Images} from 'lucide-react'
import {request,errorText} from '../../api/client'
import type {Generation,Me} from '../../types'
import {statusLabels} from '../../types'
import {Badge} from '../ui/Badge'
import {Photo} from '../ui/Photo'
export function CreationsPage(){
 const me=useOutletContext<Me>(),[status,setStatus]=useState(''),[page,setPage]=useState(1)
 const list=useQuery({queryKey:['generations',status,page],queryFn:()=>request<{items:Generation[];has_more:boolean}>('/generations?'+new URLSearchParams({status,page:String(page)})),refetchInterval:5000})
 if(me.user.role==='admin')return <Navigate to="/admin" replace/>
 return <><div className="page-heading"><div><p className="eyebrow">YOUR CREATIVE COLLECTION</p><h1>我的作品</h1><p className="muted">查看生成进度、调整文案，把素材带到你的店铺。</p></div><Link className="primary" to="/products">选择商品创作</Link></div><div className="toolbar"><select aria-label="作品状态" value={status} onChange={e=>{setStatus(e.target.value);setPage(1)}}><option value="">全部状态</option>{Object.entries(statusLabels).filter(([key])=>key!=='pending').map(([key,value])=><option key={key} value={key}>{value}</option>)}</select></div>{list.isPending?<div className="empty">正在加载作品…</div>:list.error?<div className="empty error">{errorText(list.error)}</div>:!list.data?.items.length?<div className="empty"><Images size={40}/><h2>你的第一组作品将在这里</h2><p className="muted">从商品库选择一件商品，开始生成。</p><Link className="primary" to="/products">前往商品库</Link></div>:<div className="product-grid">{list.data.items.map(g=>{const img=g.artifacts.flatMap(i=>i.versions).find(v=>v.is_selected);return <Link className="product-card creation-card" to={'/creations/'+g.id} key={g.id}><div className="product-photo"><Photo src={img?.url||g.product_image_url} alt={g.product_name}/><span className="category"><Badge status={g.status}/></span></div><div className="card-body"><h2>{g.product_name}</h2><p className="muted">{g.preset==='product_showcase'?'商品展示':'抖音带货'} · {new Date(g.created_at).toLocaleDateString()}</p><p>已结算 {g.settled_credits} 点{g.reserved_credits>0?' · 预占 '+g.reserved_credits+' 点':''}</p><span className="text-link">查看与下载 →</span></div></Link>})}</div>}<div className="pagination"><button disabled={page===1} onClick={()=>setPage(page-1)}>上一页</button><span>第 {page} 页</span><button disabled={!list.data?.has_more} onClick={()=>setPage(page+1)}>下一页</button></div></>
}
