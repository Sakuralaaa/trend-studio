import {useQuery} from '@tanstack/react-query'
import {useState} from 'react'
import {request,errorText} from '../../api/client'
import {Modal} from '../ui/Modal'
export function CreditLedgerModal({onClose}:{onClose:()=>void}){
 const [page,setPage]=useState(1)
 const ledger=useQuery({queryKey:['ledger',page],queryFn:()=>request<{items:{id:string;type:string;available_delta:number;reserved_delta:number;description:string;created_at:string}[]}>('/credits/ledger?page='+page)})
 return <Modal isOpen title="额度明细" onClose={onClose}><p className="muted">生成前预占，成功单项结算，明确失败释放；待核实的单项暂时保留预占。</p>{ledger.isPending?<p>正在加载…</p>:ledger.error?<p className="error">{errorText(ledger.error)}</p>:!ledger.data?.items.length?<p>暂无额度流水</p>:ledger.data.items.map((row,i)=><div className="notice" key={i}><div className="row"><span>{row.description}</span><strong>{row.available_delta>0?'+':''}{row.available_delta} 可用</strong></div><small>{({grant:'赠送/调整',reserve:'预占',settle:'结算',release:'退还'} as Record<string,string>)[row.type]} · 预占变化 {row.reserved_delta} · {new Date(row.created_at).toLocaleString()}</small></div>)}<div className="pagination"><button disabled={page===1} onClick={()=>setPage(page-1)}>上一页</button><span>第 {page} 页</span><button disabled={!ledger.data||ledger.data.items.length<24} onClick={()=>setPage(page+1)}>下一页</button></div></Modal>
}
