import {useState} from 'react'
import {CreditLedgerModal} from '../common/CreditLedgerModal'
import {NavLink} from 'react-router-dom'
import {Package,Radar,Images,LogOut} from 'lucide-react'
import {request,queryClient} from '../../api/client'
import type {Me} from '../../types'
const links=[{to:'/products',name:'我的商品',icon:Package},{to:'/topics',name:'今日选题',icon:Radar},{to:'/creations',name:'我的作品',icon:Images}]
export function Navbar({me}:{me:Me}){
 const [ledger,setLedger]=useState(false)
 const nav=<>{links.map(({to,name,icon:Icon})=><NavLink key={to} to={to}><Icon size={18}/><span>{name}</span></NavLink>)}</>
 return <><header className="navbar"><NavLink to={me.user.role==='admin'?'/admin':'/products'} className="brand"><span className="brand-icon">T</span><span>Trend Studio<small>电商图片与内容创作</small></span></NavLink>{me.user.role==='merchant'&&<nav className="desktop-nav">{nav}</nav>}<div className="account">{me.tenant&&<button className="credits" aria-label="查看额度明细" onClick={()=>setLedger(true)}>{me.tenant.balance} 点<small>预占 {me.tenant.reserved_balance}</small></button>}{me.user.role==='admin'&&<NavLink to="/admin">管理平台</NavLink>}<span className="account-name">{me.user.name}</span><button aria-label="退出登录" onClick={async()=>{await request('/auth/logout','POST');queryClient.clear();window.location.assign('/login')}}><LogOut size={18}/></button></div></header>{me.user.role==='merchant'&&<nav className="mobile-nav">{nav}</nav>}{ledger&&<CreditLedgerModal onClose={()=>setLedger(false)}/>}</>
}
