import {QueryClient} from '@tanstack/react-query'
export const queryClient=new QueryClient({defaultOptions:{queries:{retry:false,staleTime:15000,refetchOnWindowFocus:true}}})
export class APIError extends Error {code:string; constructor(message:string,code:string){super(message);this.code=code}}
export async function request<T>(path:string,method='GET',body?:unknown,signal?:AbortSignal):Promise<T>{
 const multipart=body instanceof FormData
 const response=await fetch('/api/v1'+path,{method,credentials:'same-origin',signal,headers:body&&!multipart?{'Content-Type':'application/json'}:{},body:body===undefined?undefined:multipart?body:JSON.stringify(body)})
 const envelope=await response.json()
 if(!response.ok||envelope.error)throw new APIError(envelope.error?.message||'服务暂不可用',envelope.error?.code||'NETWORK_ERROR')
 return envelope.data as T
}
export function refresh(...keys:string[]){for(const key of keys)void queryClient.invalidateQueries({queryKey:[key]})}
export function errorText(error:unknown){return error instanceof Error?error.message:'操作未完成，请重试'}
export async function exportGeneration(id:string){
 const response=await fetch('/api/v1/generations/'+id+'/export',{method:'POST',credentials:'same-origin'})
 if(!response.ok){const result=await response.json();throw new Error(result.error?.message||'下载失败')}
 const url=URL.createObjectURL(await response.blob());const a=document.createElement('a');a.href=url;a.download='trend-studio-'+id+'.zip';a.click();setTimeout(()=>URL.revokeObjectURL(url),60000)
}
