import {statusLabels} from '../../types'
export function Badge({status}:{status:string}){return <span className={'badge '+status}>{statusLabels[status]||status}</span>}
