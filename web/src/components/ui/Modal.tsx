import {useEffect,useId,useRef} from 'react'
import type {ReactNode} from 'react'
import {X} from 'lucide-react'
const stack:string[]=[]
let overflow=''
export function Modal({isOpen,onClose,title,children}:{isOpen:boolean;onClose:()=>void;title:string;children:ReactNode}){
 const id=useId(),dialog=useRef<HTMLDivElement>(null),close=useRef(onClose)
 useEffect(()=>{close.current=onClose},[onClose])
 useEffect(()=>{
  if(!isOpen)return
  const previous=document.activeElement as HTMLElement|null
  if(!stack.length){overflow=document.body.style.overflow;document.body.style.overflow='hidden'}stack.push(id)
  const focusables=()=>Array.from(dialog.current?.querySelectorAll<HTMLElement>('button:not(:disabled),input:not(:disabled),textarea:not(:disabled),select:not(:disabled),a[href],[tabindex="0"]')||[])
  const frame=requestAnimationFrame(()=>(focusables()[0]||dialog.current)?.focus())
  const handler=(e:KeyboardEvent)=>{if(stack.at(-1)!==id)return;if(e.key==='Escape'){e.stopPropagation();close.current()}if(e.key==='Tab'){const list=focusables(),first=list[0],last=list.at(-1);if(!list.length){e.preventDefault();dialog.current?.focus()}else if(e.shiftKey&&(document.activeElement===first||document.activeElement===dialog.current)){e.preventDefault();last?.focus()}else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first?.focus()}}}
  window.addEventListener('keydown',handler)
  return()=>{cancelAnimationFrame(frame);stack.splice(stack.indexOf(id),1);window.removeEventListener('keydown',handler);if(!stack.length)document.body.style.overflow=overflow;previous?.focus()}
 },[isOpen,id])
 if(!isOpen)return null
 return <div className="modal-backdrop" onMouseDown={e=>{if(e.target===e.currentTarget&&stack.at(-1)===id)onClose()}}><div ref={dialog} role="dialog" aria-modal="true" aria-labelledby={id} tabIndex={-1} className="modal"><header><h2 id={id}>{title}</h2><button aria-label="关闭弹窗" onClick={onClose}><X size={20}/></button></header><div className="modal-body">{children}</div></div></div>
}
