import {useState} from 'react'
import type {FormEvent} from 'react'
import {useLocation,useNavigate} from 'react-router-dom'
import {request,refresh,errorText} from '../../api/client'
import type {User} from '../../types'
export function AuthPage(){
 const location=useLocation(),navigate=useNavigate(),activate=location.pathname==='/activate',reset=location.pathname==='/reset'
 const [error,setError]=useState(''),[busy,setBusy]=useState(false)
 async function submit(event:FormEvent<HTMLFormElement>){event.preventDefault();setBusy(true);setError('');const values=Object.fromEntries(new FormData(event.currentTarget));try{const endpoint=activate?'/auth/invite':reset?'/auth/reset':'/auth/login';if(activate||reset)values.token=new URLSearchParams(location.search).get('token')||'';const result=await request<User>(endpoint,'POST',values);refresh('me');navigate(reset?'/login':result.role==='admin'?'/admin':'/products',{replace:true})}catch(error){setError(errorText(error))}finally{setBusy(false)}}
 return <main className="auth-page"><div className="auth-card"><span className="brand-icon">T</span><p className="eyebrow">TREND STUDIO</p><h1>{activate?'开启你的创作空间':reset?'设置新密码':'欢迎回来'}</h1><p className="muted">电商图片与内容创作 · 邀请内测</p><form onSubmit={submit}>{!reset&&<label>邮箱<input name="email" type="email" autoComplete="username" required/></label>}{activate&&<label>店铺名称<input name="name" maxLength={80} required/></label>}<label>密码<input name="password" type="password" minLength={activate||reset?8:1} maxLength={128} autoComplete={activate||reset?'new-password':'current-password'} required/></label>{error&&<p role="alert" className="error">{error}</p>}<button className="primary" disabled={busy}>{busy?'正在处理…':activate?'激活并进入':reset?'更新密码':'登录'}</button></form><p className="muted">没有账号或忘记密码？请联系管理员获取邀请或重置链接。</p></div></main>
}
