import {useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {Link,Navigate,useOutletContext} from 'react-router-dom'
import {Plus,Search,Package} from 'lucide-react'
import {request,refresh,errorText} from '../../api/client'
import {categories} from '../../types'
import type {Product,Me} from '../../types'
import {ProductModal} from './ProductModal'
import {Photo} from '../ui/Photo'
import {Modal} from '../ui/Modal'
import {useToast} from '../ui/Toast'
export function ProductsPage(){
 const me=useOutletContext<Me>(),toast=useToast()
 const [q,setQ]=useState(''),[category,setCategory]=useState(''),[page,setPage]=useState(1),[edit,setEdit]=useState<Product|null|undefined>(undefined),[archive,setArchive]=useState<Product>()
 const list=useQuery({queryKey:['products',q,category,page],enabled:me.user.role==='merchant',queryFn:({signal})=>request<{items:Product[];total:number}>('/products?'+new URLSearchParams({q,category,page:String(page)}),'GET',undefined,signal)})
 if(me.user.role==='admin')return <Navigate to="/admin" replace/>
 return <><div className="page-heading"><div><p className="eyebrow">PRODUCT LIBRARY</p><h1>我的商品</h1><p className="muted">上传一次商品资料，随时创作新的图片与文案。</p></div><button className="primary" onClick={()=>setEdit(null)}><Plus size={18}/>新增商品</button></div><div className="toolbar"><label className="search"><Search size={18}/><input aria-label="搜索商品" placeholder="搜索商品名称" value={q} onChange={e=>{setQ(e.target.value);setPage(1)}}/></label><select aria-label="商品类目筛选" value={category} onChange={e=>{setCategory(e.target.value);setPage(1)}}><option value="">全部类目</option>{categories.map(c=><option key={c}>{c}</option>)}</select><span className="muted">{list.data?.total??'—'} 件商品</span></div>
 {list.isPending?<div className="empty">正在加载商品…</div>:list.error?<div className="empty error">{errorText(list.error)}<button onClick={()=>void list.refetch()}>重试</button></div>:!list.data?.items.length?<div className="empty"><Package size={40}/><h2>{q||category?'没有找到商品':'上传你的第一件商品'}</h2><p className="muted">一张清晰的参考图，就可以开始创作。</p><button className="primary" onClick={()=>setEdit(null)}>上传商品</button></div>:<div className="product-grid">{list.data.items.map(p=><article className="product-card" key={p.id}><div className="product-photo"><Photo src={p.reference_images[0]?.url} alt={p.name}/><span className="category">{p.category}</span></div><div className="card-body"><h2>{p.name}</h2><p className="muted">{p.brand||'未填写品牌'} · {p.reference_images.length} 张参考图</p><div className="chips">{p.selling_points.slice(0,2).map((point,i)=><span key={i}>{point}</span>)}</div><div className="card-actions"><Link className="primary" to={'/create?product='+p.id}>生成素材</Link><button onClick={()=>setEdit(p)}>编辑</button><button aria-label={'归档'+p.name} onClick={()=>setArchive(p)}>归档</button></div></div></article>)}</div>}
 <div className="pagination"><button disabled={page===1} onClick={()=>setPage(page-1)}>上一页</button><span>第 {page} 页</span><button disabled={!list.data||page*24>=list.data.total} onClick={()=>setPage(page+1)}>下一页</button></div>
 {edit!==undefined&&<ProductModal product={edit} vision={me.capabilities.vision} onClose={()=>{setEdit(undefined);refresh('products')}}/>}
 <Modal isOpen={!!archive} title="归档商品" onClose={()=>setArchive(undefined)}><p>归档后不会出现在商品列表中，已有作品仍会保留。</p><button className="primary" onClick={async()=>{try{await request('/products/'+archive?.id,'DELETE');setArchive(undefined);refresh('products')}catch(e){toast.error(errorText(e))}}}>确认归档</button></Modal></>
}
