import {test,expect} from '@playwright/test'
import type {APIRequestContext,Page} from '@playwright/test'
import fs from 'node:fs/promises'
async function api<T>(client:APIRequestContext,path:string,data?:unknown,method='POST'):Promise<T>{const response=await client.fetch('/api/v1'+path,{method,data});const envelope=await response.json();expect(response.ok(),JSON.stringify(envelope)).toBeTruthy();return envelope.data as T}
async function screenshot(page:Page,name:string){await page.screenshot({path:'test-results/screenshots/'+name+'.png',fullPage:true})}
const reference=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a0e8AAAAASUVORK5CYII=','base64')
test('real API workflow, billing, isolation and responsive UI',async({browser})=>{
 const admin=await browser.newContext({baseURL:'http://localhost:8080'}),merchant=await browser.newContext({baseURL:'http://localhost:8080'}),other=await browser.newContext({baseURL:'http://localhost:8080'})
 await api(admin.request,'/auth/login',{email:'admin@example.test',password:'CIonlyPassword123'})
 for(const [ctx,email] of [[merchant,'merchant@example.test'],[other,'other@example.test']] as const){const invite=await api<{url:string}>(admin.request,'/admin/invites',{email,default_credits:500,name:''});await api(ctx.request,'/auth/invite',{token:new URL(invite.url).searchParams.get('token'),email,name:'日常好物店',password:'CImerchantPassword123'})}
 const page=await merchant.newPage();await page.goto('/');await expect(page).toHaveURL(/products/);await expect(page.getByRole('heading',{name:'上传你的第一件商品'})).toBeVisible()
 for(const [width,height,name] of [[1440,1000,'desktop'],[1024,768,'tablet'],[390,844,'mobile']] as const){await page.setViewportSize({width,height});await screenshot(page,name+'-empty')}
 // Exercise the actual file picker and empty form, never seed the frontend store.
 await page.getByRole('button',{name:'新增商品'}).click();await expect(page.getByLabel('商品名称 *')).toHaveValue('');await expect(page.getByLabel('类目 *')).toHaveValue('')
 await page.getByLabel('商品名称 *').fill('自然米色收纳盒');await page.getByLabel('类目 *').selectOption('家居日用')
 await page.getByLabel('商品参考图').setInputFiles({name:'box.png',mimeType:'image/png',buffer:reference});await page.getByRole('button',{name:'保存商品',exact:true}).click();await expect(page.getByRole('dialog')).not.toBeVisible()
 const first=(await api<{items:{id:string}[]}>(merchant.request,'/products',undefined,'GET')).items[0].id
 const second=await api<{id:string}>(merchant.request,'/products',{name:'秋日棉衬衫',category:'服装鞋包',brand:'',external_sku:'SKU-2',color:'米白',price:null,selling_points:['已确认的商品款式','自然色系'],usage_scenarios:[]})
 const upload=await merchant.request.post('/api/v1/products/'+second.id+'/references',{multipart:{file:{name:'shirt.png',mimeType:'image/png',buffer:reference}}});expect(upload.ok()).toBeTruthy()
 const invalid=await merchant.request.post('/api/v1/products/'+second.id+'/references',{multipart:{file:{name:'fake.png',mimeType:'image/png',buffer:Buffer.from('invalid')}}});expect(invalid.status()).toBe(400)
 for(const path of ['/products/'+second.id,'/admin/overview']){const denied=await other.request.get('/api/v1'+path);expect([403,404]).toContain(denied.status())}
 await page.reload();await expect(page.getByRole('heading',{name:'秋日棉衬衫',exact:true})).toBeVisible()
 for(const [width,height,name] of [[1440,1000,'desktop'],[1024,768,'tablet'],[390,844,'mobile']] as const){await page.setViewportSize({width,height});await screenshot(page,name+'-products')}
 await page.goto('/create?product='+second.id);await expect(page.getByRole('button',{name:/商品展示/})).toHaveAttribute('aria-pressed','true')
 // Before provider configuration, a real error is displayed, no fabricated assets.
 await expect(page.getByText('管理员尚未配置文字接口')).toBeVisible();await screenshot(page,'unconfigured')
 for(const kind of ['text','image']){await api(admin.request,'/admin/providers',{kind,name:'CI '+kind,base_url:'http://fixture:8091/v1',model:'fixture',api_key:'ci-only',image_field:'image[]',max_references:6,size:'1024x1024',timeout_seconds:10,result_hosts:[]})}
 await page.reload();await expect(page.getByRole('button',{name:'生成素材',exact:true})).toBeEnabled();await expect(page.getByRole('button',{name:/商品展示/})).toHaveAttribute('aria-pressed','true')
 await page.getByLabel('选择商品').selectOption(first);await page.getByLabel('选择商品').selectOption(second.id)
 for(const [width,height,name] of [[1440,1000,'desktop'],[1024,768,'tablet'],[390,844,'mobile']] as const){await page.setViewportSize({width,height});await screenshot(page,name+'-create')}
 await page.getByRole('button',{name:'生成素材',exact:true}).click();await expect(page).toHaveURL(/creations\//)
 const id=page.url().split('/').at(-1)!
 let generation: {status:string;product_id:string;settled_credits:number;copy_version:number;copy_data:unknown;artifacts:{type:string;versions:{id:string;url:string}[]}[]}
 await expect.poll(async()=>{generation=await api(merchant.request,'/generations/'+id,undefined,'GET');return generation.status},{timeout:90000}).toBe('succeeded')
 generation=await api(merchant.request,'/generations/'+id,undefined,'GET');expect(generation.product_id).toBe(second.id);expect(generation.settled_credits).toBe(23)
 expect((await other.request.get('/api/v1/generations/'+id)).status()).toBe(404)
 const asset=generation.artifacts.flatMap(i=>i.versions)[0];expect((await other.request.get('/api/v1/assets/'+asset.id)).status()).toBe(404)
 // Polling cannot replace an in-progress editing draft.
 await page.getByLabel('标题 1').fill('最新手动保存的长标题：自然生活中的商品灵感');await page.waitForTimeout(3500);await expect(page.getByLabel('标题 1')).toHaveValue('最新手动保存的长标题：自然生活中的商品灵感')
 const downloadPromise=page.waitForEvent('download');await page.getByRole('button',{name:'下载素材包'}).click();const download=await downloadPromise;const downloadPath=await download.path();expect(downloadPath).toBeTruthy()
 const zip=await fs.readFile(downloadPath!);expect(zip.subarray(0,2).toString()).toBe('PK')
 await expect.poll(async()=>(await api<{render_pending:boolean}>(merchant.request,'/generations/'+id,undefined,'GET')).render_pending).toBe(false)
 for(const [width,height,name] of [[1440,1000,'desktop'],[1024,768,'tablet'],[390,844,'mobile']] as const){await page.setViewportSize({width,height});await screenshot(page,name+'-detail')}
 // Concurrent duplicate submission is checked by the DB integration suite; browser/API checks same-key replay.
 const q=await api<{quote_id:string;idempotency_key:string;total_credits:number}>(merchant.request,'/generations/quote',{product_id:second.id,preset:'douyin_sales'});expect(q.total_credits).toBe(13)
 const submitted=await api<{id:string}>(merchant.request,'/generations',{quote_id:q.quote_id,idempotency_key:q.idempotency_key});const replay=await api<{id:string}>(merchant.request,'/generations',{quote_id:q.quote_id,idempotency_key:q.idempotency_key});expect(replay.id).toBe(submitted.id)
 await expect.poll(async()=>(await api<{status:string}>(merchant.request,'/generations/'+submitted.id,undefined,'GET')).status,{timeout:90000}).toBe('succeeded')
 const douyin=await api<{settled_credits:number;artifacts:{type:string}[]}>(merchant.request,'/generations/'+submitted.id,undefined,'GET');expect(douyin.settled_credits).toBe(13);expect(douyin.artifacts.some(i=>i.type==='vertical_cover')).toBeTruthy();expect(douyin.artifacts.some(i=>i.type==='main_image')).toBeFalsy()
 await page.goto('/create?product='+second.id);await expect(page.getByRole('button',{name:/商品展示/})).toHaveAttribute('aria-pressed','true')
 // Partial failure settles only successful model calls; unknown calls remain reserved.
 for(const [tag,state,cost] of [['PARTIAL','partial',13],['UNKNOWN','needs_review',13]] as const){
  const p=await api<{id:string}>(merchant.request,'/products',{name:'['+tag+'] 测试商品',category:'家居日用',selling_points:[],usage_scenarios:[]})
  expect((await merchant.request.post('/api/v1/products/'+p.id+'/references',{multipart:{file:{name:'ref.png',mimeType:'image/png',buffer:reference}}})).ok()).toBeTruthy()
  const quote=await api<{quote_id:string;idempotency_key:string}>(merchant.request,'/generations/quote',{product_id:p.id})
  const task=await api<{id:string}>(merchant.request,'/generations',{quote_id:quote.quote_id,idempotency_key:quote.idempotency_key})
  await expect.poll(async()=>(await api<{status:string}>(merchant.request,'/generations/'+task.id,undefined,'GET')).status,{timeout:90000}).toBe(state)
  const result=await api<{settled_credits:number;reserved_credits:number}>(merchant.request,'/generations/'+task.id,undefined,'GET');expect(result.settled_credits).toBe(cost);expect(result.reserved_credits).toBe(tag==='UNKNOWN'?10:0)
  await page.goto('/creations/'+task.id);await screenshot(page,tag.toLowerCase())
 }
 // Real collector asynchronous task ID is retained across polling.
 await api(admin.request,'/admin/collector',{base_url:'http://fixture:8091',api_key:'ci-only'},'PUT')
 const imported=await api<{id:string}>(merchant.request,'/topic-imports',{url:'https://www.douyin.com/video/1234567890'})
 await expect.poll(async()=>(await api<{status:string}>(merchant.request,'/topic-imports/'+imported.id,undefined,'GET')).status,{timeout:90000}).toBe('succeeded')
 await page.goto('/topics');await expect(page.getByRole('heading',{name:'秋日自然色系的日常穿搭'})).toBeVisible()
 for(const [width,height,name] of [[1440,1000,'desktop'],[1024,768,'tablet'],[390,844,'mobile']] as const){await page.setViewportSize({width,height});await screenshot(page,name+'-topics')}
 // Long content and network image failures remain inside a 360px viewport.
 const long=await api<{id:string}>(merchant.request,'/products',{name:'适合自然日常生活场景的长商品名称'.repeat(6),category:'美妆个护',selling_points:['已确认的长卖点'.repeat(15)],usage_scenarios:[]})
 expect((await merchant.request.post('/api/v1/products/'+long.id+'/references',{multipart:{file:{name:'long.png',mimeType:'image/png',buffer:reference}}})).ok()).toBeTruthy()
 await page.route('**/api/v1/assets/**',route=>route.abort())
 await page.goto('/products');await page.setViewportSize({width:360,height:844});await expect(page.getByText('图片未加载').first()).toBeVisible();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();await screenshot(page,'mobile-long-content-image-failure')
 const adminPage=await admin.newPage();await adminPage.goto('/admin');await expect(adminPage.getByRole('heading',{name:'管理平台'})).toBeVisible();for(const [width,height,name] of [[1440,1000,'desktop'],[1024,768,'tablet'],[390,844,'mobile']] as const){await adminPage.setViewportSize({width,height});await screenshot(adminPage,name+'-admin')}
 await admin.close();await merchant.close();await other.close()
})
