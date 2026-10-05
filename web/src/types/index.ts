export type Preset='product_showcase'|'douyin_sales'
export type Status='queued'|'running'|'succeeded'|'partial'|'failed'|'needs_review'|'cancelled'
export interface User {id:string; name:string; email:string; role:'merchant'|'admin'; tenant_id:string|null}
export interface Tenant {id:string; name:string; balance:number; reserved_balance:number}
export interface Me {user:User; tenant:Tenant|null; capabilities:{vision:boolean}}
export interface Reference {id:string; url:string; file_name:string; is_primary:boolean; sort_order:number}
export interface Product {id:string; name:string; category:string; brand:string; external_sku:string; color:string; price:number|null; selling_points:string[]; usage_scenarios:string[]; reference_images:Reference[]; version:number; status:string}
export interface Topic {id:string; title:string; description:string; author:string; category:string; original_url:string; crawled_at:string; weighted_interaction:number|null; hourly_growth:number|null; trend_score:number; status_tag:string; match_score?:number; match_reason?:string; hook:string; angle_summary:string}
export interface QuoteItem {item_type:string; name:string; credits:number; provider_type:string}
export interface Quote {quote_id:string; product_id:string; product_name:string; preset:Preset; topic_id?:string; total_credits:number; items:QuoteItem[]; expires_at:string; idempotency_key:string}
export interface CopyData {titles:string[]; description:string; selling_points:string[]; script?:{text:string; estimated_seconds:number}; caption?:{text:string; tags:string[]}}
export interface Asset {id:string; url:string; is_selected:boolean; width:number; height:number; copy_version:number; created_at:string; label:string}
export interface Item {id:string; type:string; name:string; status:string; credits:number; error_message:string|null; versions:Asset[]; provider_type:string}
export interface Generation {id:string; product_id:string; product_name:string; product_image_url:string; preset:Preset; status:Status; copy_data:CopyData; copy_version:number; render_pending:boolean; reserved_credits:number; settled_credits:number; released_credits:number; artifacts:Item[]; created_at:string; input_snapshot:{product:Product}}
export interface Provider {id?:string; kind:string; name:string; base_url:string; model:string; api_key?:string; configured?:boolean; image_field:string; max_references:number; size:string; timeout_seconds:number; vision_model?:string; result_hosts:string[]}
export interface Source {id?:string; account_name:string; url:string; category:string; active:boolean; last_crawled_at?:string; last_error?:string}
export const categories=['服装鞋包','美妆个护','家居日用','食品饮料','数码家电','母婴','运动户外','其他']
export const statusLabels:Record<string,string>={queued:'等待生成',running:'生成中',succeeded:'已完成',partial:'部分完成',failed:'生成失败',needs_review:'待核实',cancelled:'已取消',pending:'等待中'}
