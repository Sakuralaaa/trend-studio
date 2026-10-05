import React, { useState, useEffect } from 'react'
import {
  CostMetrics,
  Invitation,
  ProviderConfig,
  AbnormalTask,
  CreditTransaction,
} from '../../types'
import { api, globalStore } from '../../api/client'
import { useToast } from '../ui/Toast'
import { Modal } from '../ui/Modal'
import {
  ShieldCheck,
  Activity,
  KeyRound,
  SlidersHorizontal,
  AlertTriangle,
  Coins,
  RefreshCw,
  Plus,
  Copy,
  CheckCircle2,
  XCircle,
  Clock,
  Zap,
  TrendingUp,
  Cpu,
  RotateCcw,
  Check,
  Sparkles,
} from 'lucide-react'

export const AdminPage: React.FC = () => {
  const { success, error, info } = useToast()

  const [activeTab, setActiveTab] = useState<'metrics' | 'invites' | 'providers' | 'abnormal'>('metrics')

  const [metrics, setMetrics] = useState<CostMetrics | null>(null)
  const [invitations, setInvitations] = useState<Invitation[]>([])
  const [providers, setProviders] = useState<ProviderConfig[]>([])
  const [abnormalTasks, setAbnormalTasks] = useState<AbnormalTask[]>([])
  const [transactions, setTransactions] = useState<CreditTransaction[]>([])
  const [loading, setLoading] = useState(false)

  // 邀请码弹窗
  const [isInviteModalOpen, setIsInviteModalOpen] = useState(false)
  const [newInviteMaxUses, setNewInviteMaxUses] = useState(1)
  const [newInviteCredits, setNewInviteCredits] = useState(100)

  // 调账弹窗
  const [isAdjustModalOpen, setIsAdjustModalOpen] = useState(false)
  const [adjustAmount, setAdjustAmount] = useState<number>(50)
  const [adjustReason, setAdjustReason] = useState('')
  const [isAdjusting, setIsAdjusting] = useState(false)

  // 测速中状态
  const [testingProviderId, setTestingProviderId] = useState<string | null>(null)

  const loadData = async () => {
    try {
      setLoading(true)
      const [mRes, iRes, pRes, aRes, tRes] = await Promise.all([
        api.admin.getMetrics(),
        api.admin.getInvitations(),
        api.admin.getProviders(),
        api.admin.getAbnormalTasks(),
        api.admin.getTransactions(),
      ])
      setMetrics(mRes.data)
      setInvitations(iRes.data)
      setProviders(pRes.data)
      setAbnormalTasks(aRes.data)
      setTransactions(tRes.data)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
    const unsubscribe = globalStore.subscribe(() => {
      setMetrics({ ...globalStore.metrics })
      setInvitations([...globalStore.invitations])
      setProviders([...globalStore.providers])
      setAbnormalTasks([...globalStore.abnormalTasks])
      setTransactions([...globalStore.transactions])
    })
    return () => unsubscribe()
  }, [])

  // 生成新邀请码
  const handleCreateInvite = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const res = await api.admin.createInvitation(newInviteMaxUses, newInviteCredits)
      success(`成功生成邀请码：${res.data.code}`)
      setIsInviteModalOpen(false)
      setInvitations([...globalStore.invitations])
    } catch (err: any) {
      error(err.message || '生成失败')
    }
  }

  // 调整额度
  const handleAdjustCredits = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!adjustReason.trim()) {
      error('请必须填写调账原因以备审计')
      return
    }
    try {
      setIsAdjusting(true)
      await api.admin.adjustCredits(globalStore.tenant.id, adjustAmount, adjustReason.trim())
      success(`成功调整额度 ${adjustAmount > 0 ? '+' : ''}${adjustAmount} 点，已记入流水`)
      setIsAdjustModalOpen(false)
      setAdjustReason('')
    } catch (err: any) {
      error(err.message || '调账失败')
    } finally {
      setIsAdjusting(false)
    }
  }

  // 测试模型提供商
  const handleTestProvider = async (id: string) => {
    try {
      setTestingProviderId(id)
      const res = await api.admin.testProvider(id)
      success(`测试成功，响应延迟 ${res.data.latency_ms}ms`)
    } catch (err: any) {
      error('通道测试超时或失败')
    } finally {
      setTestingProviderId(null)
    }
  }

  // 切换提供商开关
  const handleToggleProvider = async (provider: ProviderConfig) => {
    try {
      await api.admin.updateProvider(provider.id, { is_active: !provider.is_active })
      success(`已${!provider.is_active ? '启用' : '禁用'}提供商通道: ${provider.name}`)
    } catch (err: any) {
      error('更新状态失败')
    }
  }

  // 干预异常任务
  const handleResolveAbnormal = async (id: string, action: 'release_credits' | 'mark_succeeded') => {
    try {
      await api.admin.resolveAbnormalTask(id, action)
      success(action === 'release_credits' ? '已原路释放预占额度给商家' : '已标记为处理完成')
    } catch (err: any) {
      error('操作失败')
    }
  }

  const copyToClipboard = (text: string, msg: string) => {
    navigator.clipboard.writeText(text)
    success(msg)
  }

  return (
    <div className=\"space-y-6\">
      {/* 顶部标题栏 */}
      <div className=\"flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-gray-100 pb-5\">
        <div>
          <div className=\"flex items-center gap-2\">
            <div className=\"w-7 h-7 rounded-lg bg-purple-100 text-purple-700 flex items-center justify-center\">
              <ShieldCheck className=\"w-4 h-4\" />
            </div>
            <h1 className=\"text-2xl font-bold tracking-tight text-gray-900\">平台运营与风控后台</h1>
          </div>
          <p className=\"text-sm text-gray-500 mt-1\">
            种子期商家邀请码管理、模型路由监控、成本核算与卡单额度人工干预
          </p>
        </div>

        <div className=\"flex items-center gap-2\">
          <button
            onClick={loadData}
            className=\"inline-flex items-center gap-1.5 px-3 py-2 text-xs font-medium text-gray-600 bg-white border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors shadow-2xs\"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            刷新
          </button>
          <button
            onClick={() => setIsAdjustModalOpen(true)}
            className=\"inline-flex items-center gap-1.5 px-3.5 py-2 text-xs font-semibold text-gray-700 bg-white border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors shadow-2xs\"
          >
            <Coins className=\"w-3.5 h-3.5 text-amber-500\" />
            商家额度调账
          </button>
          <button
            onClick={() => setIsInviteModalOpen(true)}
            className=\"inline-flex items-center gap-1.5 px-3.5 py-2 text-xs font-semibold text-white bg-purple-600 rounded-lg hover:bg-purple-700 transition-colors shadow-sm\"
          >
            <Plus className=\"w-3.5 h-3.5\" />
            生成新邀请码
          </button>
        </div>
      </div>

      {/* 关键大盘指标 */}
      {metrics && (
        <div className=\"grid grid-cols-2 md:grid-cols-4 gap-4\">
          <div className=\"p-4 rounded-xl bg-white border border-gray-100 shadow-2xs\">
            <div className=\"text-xs text-gray-500 flex items-center justify-between\">
              <span>今日总生成次数</span>
              <Activity className=\"w-3.5 h-3.5 text-indigo-500\" />
            </div>
            <div className=\"text-2xl font-bold text-gray-900 mt-1.5\">{metrics.total_tasks_today}</div>
            <div className=\"text-[11px] text-emerald-600 mt-1 flex items-center gap-1\">
              <TrendingUp className=\"w-3 h-3\" />
              成功率 {metrics.success_rate}%
            </div>
          </div>

          <div className=\"p-4 rounded-xl bg-white border border-gray-100 shadow-2xs\">
            <div className=\"text-xs text-gray-500 flex items-center justify-between\">
              <span>模型消耗实际成本</span>
              <Coins className=\"w-3.5 h-3.5 text-amber-500\" />
            </div>
            <div className=\"text-2xl font-bold text-gray-900 mt-1.5\">¥{metrics.total_cost_today.toFixed(2)}</div>
            <div className=\"text-[11px] text-gray-500 mt-1\">
              额度消耗 {metrics.total_credits_spent} credits
            </div>
          </div>

          <div className=\"p-4 rounded-xl bg-white border border-gray-100 shadow-2xs\">
            <div className=\"text-xs text-gray-500 flex items-center justify-between\">
              <span>平均端到端耗时</span>
              <Clock className=\"w-3.5 h-3.5 text-blue-500\" />
            </div>
            <div className=\"text-2xl font-bold text-gray-900 mt-1.5\">{metrics.avg_duration_sec}s</div>
            <div className=\"text-[11px] text-emerald-600 mt-1\">P95 延迟受控在 22s 内</div>
          </div>

          <div className=\"p-4 rounded-xl bg-white border border-gray-100 shadow-2xs\">
            <div className=\"text-xs text-gray-500 flex items-center justify-between\">
              <span>卡单与异常告警</span>
              <AlertTriangle className=\"w-3.5 h-3.5 text-red-500\" />
            </div>
            <div className=\"text-2xl font-bold text-gray-900 mt-1.5\">
              {abnormalTasks.filter((a) => a.status === 'open').length}
            </div>
            <div className=\"text-[11px] text-gray-500 mt-1\">需要人工判定或退额</div>
          </div>
        </div>
      )}

      {/* 后台分栏导航 */}
      <div className=\"flex border-b border-gray-200 space-x-6 text-sm font-medium\">
        <button
          onClick={() => setActiveTab('metrics')}
          className={`pb-3 border-b-2 transition-colors flex items-center gap-1.5 ${
            activeTab === 'metrics'
              ? 'border-purple-600 text-purple-600 font-semibold'
              : 'border-transparent text-gray-500 hover:text-gray-700'
          }`}
        >
          <Activity className=\"w-4 h-4\" />
          大盘与不可篡改账本
        </button>

        <button
          onClick={() => setActiveTab('invites')}
          className={`pb-3 border-b-2 transition-colors flex items-center gap-1.5 ${
            activeTab === 'invites'
              ? 'border-purple-600 text-purple-600 font-semibold'
              : 'border-transparent text-gray-500 hover:text-gray-700'
          }`}
        >
          <KeyRound className=\"w-4 h-4\" />
          商家邀请码管理 ({invitations.length})
        </button>

        <button
          onClick={() => setActiveTab('providers')}
          className={`pb-3 border-b-2 transition-colors flex items-center gap-1.5 ${
            activeTab === 'providers'
              ? 'border-purple-600 text-purple-600 font-semibold'
              : 'border-transparent text-gray-500 hover:text-gray-700'
          }`}
        >
          <Cpu className=\"w-4 h-4\" />
          模型通道与熔断路由 ({providers.length})
        </button>

        <button
          onClick={() => setActiveTab('abnormal')}
          className={`pb-3 border-b-2 transition-colors flex items-center gap-1.5 ${
            activeTab === 'abnormal'
              ? 'border-purple-600 text-purple-600 font-semibold'
              : 'border-transparent text-gray-500 hover:text-gray-700'
          }`}
        >
          <AlertTriangle className=\"w-4 h-4\" />
          异常任务干预 ({abnormalTasks.filter((a) => a.status === 'open').length})
        </button>
      </div>

      {/* Tab 1: 大盘与不可篡改账本 */}
      {activeTab === 'metrics' && (
        <div className=\"space-y-4\">
          <div className=\"bg-white rounded-xl border border-gray-100 p-5 shadow-2xs\">
            <div className=\"flex items-center justify-between mb-4\">
              <div>
                <h3 className=\"text-sm font-bold text-gray-900\">全局额度变动审计流水 (Immutable Ledger)</h3>
                <p className=\"text-xs text-gray-500 mt-0.5\">
                  严格遵循预占 (Reserve) → 结算 (Settle) → 释放 (Release) 三阶段不可篡改会计账本
                </p>
              </div>
            </div>

            <div className=\"overflow-x-auto\">
              <table className=\"w-full text-left text-xs\">
                <thead className=\"bg-gray-50 text-gray-500 uppercase text-[10px]\">
                  <tr>
                    <th className=\"px-4 py-2.5 rounded-l-lg\">流水ID / 时间</th>
                    <th className=\"px-4 py-2.5\">动作类型</th>
                    <th className=\"px-4 py-2.5\">变动额度</th>
                    <th className=\"px-4 py-2.5\">变动后余额</th>
                    <th className=\"px-4 py-2.5\">业务关联说明</th>
                    <th className=\"px-4 py-2.5 rounded-r-lg\">关联单号</th>
                  </tr>
                </thead>
                <tbody className=\"divide-y divide-gray-100\">
                  {transactions.slice(0, 10).map((tx) => {
                    const isPositive = tx.amount > 0
                    return (
                      <tr key={tx.id} className=\"hover:bg-gray-50/60\">
                        <td className=\"px-4 py-3\">
                          <div className=\"font-mono text-[11px] text-gray-700\">{tx.id}</div>
                          <div className=\"text-[10px] text-gray-400 mt-0.5\">
                            {new Date(tx.created_at).toLocaleTimeString('zh-CN')}
                          </div>
                        </td>
                        <td className=\"px-4 py-3\">
                          <span
                            className={`px-2 py-0.5 rounded text-[10px] font-semibold ${
                              tx.type === 'grant'
                                ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                                : tx.type === 'reserve'
                                ? 'bg-amber-50 text-amber-700 border border-amber-200'
                                : tx.type === 'settle'
                                ? 'bg-indigo-50 text-indigo-700 border border-indigo-200'
                                : 'bg-blue-50 text-blue-700 border border-blue-200'
                            }`}
                          >
                            {tx.type === 'grant'
                              ? '后台发放'
                              : tx.type === 'reserve'
                              ? '任务预占'
                              : tx.type === 'settle'
                              ? '确认结算'
                              : '取消退还'}
                          </span>
                        </td>
                        <td className=\"px-4 py-3 font-semibold font-mono\">
                          <span className={isPositive ? 'text-emerald-600' : 'text-gray-900'}>
                            {isPositive ? `+${tx.amount}` : tx.amount}
                          </span>
                        </td>
                        <td className=\"px-4 py-3 text-gray-600 font-mono\">{tx.balance_after}</td>
                        <td className=\"px-4 py-3 text-gray-600 max-w-xs truncate\">{tx.description}</td>
                        <td className=\"px-4 py-3 text-gray-400 font-mono text-[11px]\">{tx.reference_id}</td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* Tab 2: 商家邀请码管理 */}
      {activeTab === 'invites' && (
        <div className=\"bg-white rounded-xl border border-gray-100 shadow-2xs overflow-hidden\">
          <div className=\"p-4 border-b border-gray-100 flex items-center justify-between\">
            <div>
              <h3 className=\"text-sm font-bold text-gray-900\">种子期邀请码发放列表</h3>
              <p className=\"text-xs text-gray-500 mt-0.5\">
                当前平台仅允许持有激活码的优质商家入驻，防范模型算力恶意滥用
              </p>
            </div>
            <button
              onClick={() => setIsInviteModalOpen(true)}
              className=\"inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-white bg-purple-600 rounded-lg hover:bg-purple-700 transition-colors shadow-2xs\"
            >
              <Plus className=\"w-3.5 h-3.5\" />
              新建邀请码
            </button>
          </div>

          <div className=\"overflow-x-auto\">
            <table className=\"w-full text-left text-xs\">
              <thead className=\"bg-gray-50 text-gray-500 uppercase text-[10px]\">
                <tr>
                  <th className=\"px-4 py-3\">邀请码 / 链接</th>
                  <th className=\"px-4 py-3\">初始额度</th>
                  <th className=\"px-4 py-3\">使用进度</th>
                  <th className=\"px-4 py-3\">有效期至</th>
                  <th className=\"px-4 py-3\">状态</th>
                  <th className=\"px-4 py-3 text-right\">操作</th>
                </tr>
              </thead>
              <tbody className=\"divide-y divide-gray-100\">
                {invitations.map((inv) => (
                  <tr key={inv.id} className=\"hover:bg-gray-50/60\">
                    <td className=\"px-4 py-3\">
                      <div className=\"flex items-center gap-2\">
                        <span className=\"font-mono font-bold text-gray-900 text-sm tracking-wide bg-gray-100 px-2 py-0.5 rounded border border-gray-200\">
                          {inv.code}
                        </span>
                        <button
                          onClick={() => copyToClipboard(inv.code, `已复制邀请码: ${inv.code}`)}
                          className=\"text-gray-400 hover:text-gray-600\"
                          title=\"复制邀请码\"
                        >
                          <Copy className=\"w-3.5 h-3.5\" />
                        </button>
                      </div>
                      <div className=\"text-[11px] text-gray-400 mt-1 truncate max-w-xs\">
                        {inv.invite_url}
                      </div>
                    </td>
                    <td className=\"px-4 py-3\">
                      <span className=\"font-semibold text-amber-600 flex items-center gap-1\">
                        <Coins className=\"w-3.5 h-3.5\" />
                        {inv.default_credits} 点
                      </span>
                    </td>
                    <td className=\"px-4 py-3\">
                      <div className=\"flex items-center gap-2\">
                        <div className=\"w-20 bg-gray-200 h-1.5 rounded-full overflow-hidden\">
                          <div
                            className=\"bg-purple-600 h-full rounded-full\"
                            style={{ width: `${Math.min(100, (inv.used_count / inv.max_uses) * 100)}%` }}
                          />
                        </div>
                        <span className=\"text-[11px] text-gray-600 font-mono\">
                          {inv.used_count}/{inv.max_uses}
                        </span>
                      </div>
                    </td>
                    <td className=\"px-4 py-3 text-gray-500\">
                      {new Date(inv.expires_at).toLocaleDateString('zh-CN')}
                    </td>
                    <td className=\"px-4 py-3\">
                      <span
                        className={`px-2 py-0.5 rounded-full text-[10px] font-semibold ${
                          inv.status === 'active'
                            ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                            : 'bg-gray-100 text-gray-500'
                        }`}
                      >
                        {inv.status === 'active' ? '生效中' : '已失效'}
                      </span>
                    </td>
                    <td className=\"px-4 py-3 text-right\">
                      <button
                        onClick={() => copyToClipboard(inv.invite_url, '已复制邀请链接')}
                        className=\"text-xs font-semibold text-purple-600 hover:text-purple-700\"
                      >
                        复制链接
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Tab 3: 模型通道与熔断路由 */}
      {activeTab === 'providers' && (
        <div className=\"space-y-4\">
          <div className=\"grid grid-cols-1 md:grid-cols-2 gap-4\">
            {providers.map((prov) => {
              const isTesting = testingProviderId === prov.id
              return (
                <div
                  key={prov.id}
                  className={`p-4 rounded-xl border bg-white shadow-2xs transition-all ${
                    prov.is_active ? 'border-gray-200' : 'border-gray-200/50 opacity-70 bg-gray-50/50'
                  }`}
                >
                  <div className=\"flex items-start justify-between\">
                    <div>
                      <div className=\"flex items-center gap-2\">
                        <h4 className=\"font-bold text-sm text-gray-900\">{prov.name}</h4>
                        <span
                          className={`text-[10px] px-2 py-0.5 rounded-full font-semibold ${
                            prov.status === 'healthy'
                              ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                              : prov.status === 'degraded'
                              ? 'bg-amber-50 text-amber-700 border border-amber-200'
                              : 'bg-red-50 text-red-700 border border-red-200'
                          }`}
                        >
                          {prov.status === 'healthy'
                            ? '服务正常'
                            : prov.status === 'degraded'
                            ? '延迟偏高'
                            : '通道熔断'}
                        </span>
                      </div>
                      <p className=\"text-xs text-gray-500 mt-1\">
                        类型: {prov.type === 'text' ? '文本模型' : '图像模型'} · 优先级: P{prov.priority}
                      </p>
                    </div>

                    <div className=\"flex items-center gap-2\">
                      <button
                        onClick={() => handleToggleProvider(prov)}
                        className={`text-xs px-2.5 py-1 rounded-md font-medium transition-colors ${
                          prov.is_active
                            ? 'bg-emerald-50 text-emerald-700 hover:bg-emerald-100'
                            : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                        }`}
                      >
                        {prov.is_active ? '主路已启用' : '已停用'}
                      </button>
                    </div>
                  </div>

                  <div className=\"grid grid-cols-3 gap-2 mt-4 pt-3 border-t border-gray-100 text-center text-xs\">
                    <div>
                      <div className=\"text-[10px] text-gray-400\">单次平均耗时</div>
                      <div className=\"font-semibold text-gray-900 mt-0.5\">{prov.avg_latency_ms} ms</div>
                    </div>
                    <div>
                      <div className=\"text-[10px] text-gray-400\">今日错误率</div>
                      <div
                        className={`font-semibold mt-0.5 ${
                          prov.error_rate > 3 ? 'text-red-600' : 'text-gray-900'
                        }`}
                      >
                        {prov.error_rate}%
                      </div>
                    </div>
                    <div>
                      <div className=\"text-[10px] text-gray-400\">最近测速</div>
                      <div className=\"font-semibold text-emerald-600 mt-0.5\">
                        {prov.last_test_status || '正常'}
                      </div>
                    </div>
                  </div>

                  <div className=\"mt-3 pt-3 border-t border-gray-100 flex items-center justify-between\">
                    <span className=\"text-[11px] text-gray-400\">
                      降级路由备选: {prov.fallback_provider_id || '无 (直接提示重试)'}
                    </span>
                    <button
                      onClick={() => handleTestProvider(prov.id)}
                      disabled={isTesting}
                      className=\"inline-flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-700 font-semibold disabled:opacity-50\"
                    >
                      <Zap className={`w-3.5 h-3.5 ${isTesting ? 'animate-bounce text-amber-500' : ''}`} />
                      {isTesting ? '测速中...' : '发起真实Ping测速'}
                    </button>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Tab 4: 异常任务干预 */}
      {activeTab === 'abnormal' && (
        <div className=\"bg-white rounded-xl border border-gray-100 shadow-2xs overflow-hidden\">
          <div className=\"p-4 border-b border-gray-100\">
            <h3 className=\"text-sm font-bold text-gray-900\">异常超时与卡单拦截队列</h3>
            <p className=\"text-xs text-gray-500 mt-0.5\">
              针对执行超过 180 秒、或第三方生图接口返回超时的异常任务，管理员可一键原路释放冻结额度
            </p>
          </div>

          <div className=\"divide-y divide-gray-100\">
            {abnormalTasks.length === 0 ? (
              <div className=\"text-center py-12 text-gray-400 text-xs\">当前无异常任务，系统运行良好</div>
            ) : (
              abnormalTasks.map((ab) => (
                <div key={ab.id} className=\"p-4 flex flex-col md:flex-row md:items-center justify-between gap-4\">
                  <div className=\"space-y-1\">
                    <div className=\"flex items-center gap-2\">
                      <span className=\"font-bold text-gray-900 text-sm\">任务 #{ab.task_id}</span>
                      <span
                        className={`text-[10px] px-2 py-0.5 rounded font-semibold ${
                          ab.status === 'open'
                            ? 'bg-red-50 text-red-700 border border-red-200'
                            : 'bg-gray-100 text-gray-500'
                        }`}
                      >
                        {ab.status === 'open' ? '待处理' : '已处理退还'}
                      </span>
                    </div>
                    <div className=\"text-xs text-gray-600\">
                      商家ID: <span className=\"font-mono\">{ab.tenant_id}</span> · 异常原因:{' '}
                      <span className=\"text-red-600 font-medium\">{ab.error_reason}</span>
                    </div>
                    <div className=\"text-[11px] text-gray-400\">
                      冻结额度: <span className=\"font-semibold text-gray-900\">{ab.reserved_credits} 点</span> ·
                      触发时间: {new Date(ab.created_at).toLocaleString('zh-CN')}
                    </div>
                  </div>

                  {ab.status === 'open' ? (
                    <div className=\"flex items-center gap-2\">
                      <button
                        onClick={() => handleResolveAbnormal(ab.id, 'release_credits')}
                        className=\"inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg transition-colors shadow-2xs\"
                      >
                        <RotateCcw className=\"w-3.5 h-3.5\" />
                        原路释放预占额度
                      </button>
                      <button
                        onClick={() => handleResolveAbnormal(ab.id, 'mark_succeeded')}
                        className=\"inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-gray-600 bg-gray-100 hover:bg-gray-200 rounded-lg transition-colors\"
                      >
                        <Check className=\"w-3.5 h-3.5\" />
                        标记忽略
                      </button>
                    </div>
                  ) : (
                    <span className=\"text-xs text-emerald-600 font-medium flex items-center gap-1\">
                      <CheckCircle2 className=\"w-4 h-4\" />
                      已成功退还商家预占额度
                    </span>
                  )}
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* 生成邀请码弹窗 */}
      <Modal
        isOpen={isInviteModalOpen}
        onClose={() => setIsInviteModalOpen(false)}
        title=\"生成种子商家邀请码\"
        description=\"创建包含专属初始额度的一次性或多次使用的入驻邀请码\"
        maxWidth=\"max-w-md\"
      >
        <form onSubmit={handleCreateInvite} className=\"space-y-4\">
          <div>
            <label className=\"block text-xs font-semibold text-gray-700 mb-1\">
              允许激活的最大商家数 (使用次数)
            </label>
            <input
              type=\"number\"
              min={1}
              max={100}
              value={newInviteMaxUses}
              onChange={(e) => setNewInviteMaxUses(Number(e.target.value))}
              className=\"w-full px-3 py-2 text-xs rounded-lg border border-gray-200 focus:outline-none focus:ring-2 focus:ring-purple-500/20 focus:border-purple-500\"
            />
          </div>

          <div>
            <label className=\"block text-xs font-semibold text-gray-700 mb-1\">
              激活后赠送初始额度 (Credits)
            </label>
            <input
              type=\"number\"
              min={10}
              step={10}
              value={newInviteCredits}
              onChange={(e) => setNewInviteCredits(Number(e.target.value))}
              className=\"w-full px-3 py-2 text-xs rounded-lg border border-gray-200 focus:outline-none focus:ring-2 focus:ring-purple-500/20 focus:border-purple-500\"
            />
            <p className=\"text-[11px] text-gray-400 mt-1\">
              注：标准商品展示整包需 23 点，抖音带货整包需 13 点。
            </p>
          </div>

          <div className=\"pt-2 flex justify-end gap-2\">
            <button
              type=\"button\"
              onClick={() => setIsInviteModalOpen(false)}
              className=\"px-3.5 py-2 text-xs font-medium text-gray-600 hover:bg-gray-100 rounded-lg transition-colors\"
            >
              取消
            </button>
            <button
              type=\"submit\"
              className=\"px-4 py-2 text-xs font-semibold text-white bg-purple-600 hover:bg-purple-700 rounded-lg transition-colors shadow-2xs\"
            >
              生成并存入列表
            </button>
          </div>
        </form>
      </Modal>

      {/* 调账弹窗 */}
      <Modal
        isOpen={isAdjustModalOpen}
        onClose={() => setIsAdjustModalOpen(false)}
        title=\"商家额度手动调账\"
        description=\"对指定商家租户进行额度增减，必须录入审计备注\"
        maxWidth=\"max-w-md\"
      >
        <form onSubmit={handleAdjustCredits} className=\"space-y-4\">
          <div>
            <label className=\"block text-xs font-semibold text-gray-700 mb-1\">
              目标商家租户
            </label>
            <input
              type=\"text\"
              disabled
              value={`${globalStore.tenant.name} (${globalStore.tenant.code})`}
              className=\"w-full px-3 py-2 text-xs bg-gray-50 rounded-lg border border-gray-200 text-gray-500 font-mono\"
            />
          </div>

          <div>
            <label className=\"block text-xs font-semibold text-gray-700 mb-1\">
              调整额度 (支持负数扣减，如 -20 或 50)
            </label>
            <input
              type=\"number\"
              step={1}
              value={adjustAmount}
              onChange={(e) => setAdjustAmount(Number(e.target.value))}
              className=\"w-full px-3 py-2 text-xs rounded-lg border border-gray-200 focus:outline-none focus:ring-2 focus:ring-purple-500/20 focus:border-purple-500 font-mono\"
            />
          </div>

          <div>
            <label className=\"block text-xs font-semibold text-gray-700 mb-1\">
              调账理由 / 审计说明 (必填)
            </label>
            <textarea
              rows={3}
              placeholder=\"例如：补偿第三方模型服务短暂网络波动、体验名额赠送等...\"
              value={adjustReason}
              onChange={(e) => setAdjustReason(e.target.value)}
              className=\"w-full px-3 py-2 text-xs rounded-lg border border-gray-200 focus:outline-none focus:ring-2 focus:ring-purple-500/20 focus:border-purple-500\"
            />
          </div>

          <div className=\"pt-2 flex justify-end gap-2\">
            <button
              type=\"button\"
              onClick={() => setIsAdjustModalOpen(false)}
              className=\"px-3.5 py-2 text-xs font-medium text-gray-600 hover:bg-gray-100 rounded-lg transition-colors\"
            >
              取消
            </button>
            <button
              type=\"submit\"
              disabled={isAdjusting}
              className=\"px-4 py-2 text-xs font-semibold text-white bg-purple-600 hover:bg-purple-700 rounded-lg transition-colors shadow-2xs disabled:opacity-50\"
            >
              确认调账并记入账本
            </button>
          </div>
        </form>
      </Modal>
    </div>
  )
}
