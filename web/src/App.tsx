import {useQuery} from '@tanstack/react-query'
import {BrowserRouter,Routes,Route,Navigate,Outlet} from 'react-router-dom'
import {request,errorText,APIError} from './api/client'
import type {Me} from './types'
import {Navbar} from './components/layout/Navbar'
import {AuthPage} from './components/auth/AuthModal'
import {ProductsPage} from './components/products/ProductsPage'
import {GenerationStudio} from './components/studio/GenerationStudio'
import {CreationsPage} from './components/creations/CreationsPage'
import {TaskDetailModal} from './components/creations/TaskDetailModal'
import {TopicsPage} from './components/topics/TopicsPage'
import {AdminPage} from './components/admin/AdminPage'
import './App.css'
function Layout(){
 const me=useQuery({queryKey:['me'],queryFn:({signal})=>request<Me>('/auth/me','GET',undefined,signal),refetchInterval:15000})
 if(me.isPending)return <div className="loading">正在加载你的空间…</div>
 if(me.error instanceof APIError&&me.error.code==='UNAUTHENTICATED')return <Navigate to="/login" replace/>
 if(!me.data)return <div className="loading error">{errorText(me.error)}<button onClick={()=>void me.refetch()}>重新加载</button></div>
 return <><Navbar me={me.data}/><main className="workspace"><Outlet context={me.data}/></main></>
}
export default function App(){return <BrowserRouter><Routes><Route path="/login" element={<AuthPage/>}/><Route path="/activate" element={<AuthPage/>}/><Route path="/reset" element={<AuthPage/>}/><Route element={<Layout/>}><Route index element={<Navigate to="/products" replace/>}/><Route path="/products" element={<ProductsPage/>}/><Route path="/topics" element={<TopicsPage/>}/><Route path="/create" element={<GenerationStudio/>}/><Route path="/creations" element={<CreationsPage/>}/><Route path="/creations/:id" element={<TaskDetailModal/>}/><Route path="/admin" element={<AdminPage/>}/></Route><Route path="*" element={<Navigate to="/products" replace/>}/></Routes></BrowserRouter>}
