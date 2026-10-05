import React, { useState, useEffect } from 'react'
import { TrendingTopic, Product, PRODUCT_CATEGORIES, ProductCategory } from '../../types'
import { globalStore } from '../../api/client'
import { TopicImportModal } from './TopicImportModal'
import { CategoryBadge, StatusBadge } from '../ui/Badge'
import {
  TrendingUp,
  Link2,
  ThumbsUp,
  MessageCircle,
  Share2,
  Bookmark,
  Sparkles,
  ExternalLink,
  Wand2,
  Radio,
  Flame,
  Lightbulb,
} from 'lucide-react'

interface TopicsPageProps {
  onSelectTopicForGeneration: (topic: TrendingTopic, product?: Product) => void
}

export const TopicsPage: React.FC<TopicsPageProps> = ({ onSelectTopicForGeneration }) => {
  const [topics, setTopics] = useState<TrendingTopic[]>([])
  const [products, setProducts] = useState<Product[]>([])
  const [selectedProduct, setSelectedProduct] = useState<Product | null>(null)
  const [selectedCategory, setSelectedCategory] = useState<string>('all')
  const [importModalOpen, setImportModalOpen] = useState(false)

  const loadData = () => {
    setTopics([...globalStore.topics])
    setProducts([...globalStore.products])
    if (!selectedProduct && globalStore.products.length > 0) {
      setSelectedProduct(globalStore.products[0])
    }
  }

  useEffect(() => {
    loadData()
    return globalStore.subscribe(loadData)
  }, [])

  // Filter topics
  const displayedTopics = topics
    .filter((t) => {
      if (selectedCategory === 'all') return true
      return t.category === selectedCategory
    })
    .sort((a, b) => b.weighted_interaction - a.weighted_interaction)

  return (
    <div className="flex flex-col gap-6 pb-16">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-neutral-900">
              今日选题
            </h1>
            <span className="text-xs font-mono font-medium bg-neutral-100 text-neutral-600 px-2 py-0.5 rounded-full flex items-center gap-1">
              <Radio className="w-3 h-3 text-emerald-500 animate-pulse" />
              基于平台监控账号
            </span>
          </div>
          <p className="mt-1 text-xs sm:text-sm text-neutral-500">
            行业热门选题与趋势榜单。加权互动指数 = 点赞＋2×评论＋3×分享＋2×收藏。
          </p>
        </div>

        <button
          onClick={() => setImportModalOpen(true)}
          className="inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-white border border-neutral-200 hover:border-neutral-900 text-neutral-900 font-semibold text-xs shadow-2xs transition-colors shrink-0"
        >
          <Link2 className="w-4 h-4 text-neutral-500" />
          <span>导入抖音链接</span>
        </button>
      </div>

      {/* Product Smart Match Bar */}
      <div className="p-4 rounded-2xl bg-neutral-900 text-white shadow-md flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-neutral-800 flex items-center justify-center text-amber-400 shrink-0">
            <Sparkles className="w-5 h-5" />
          </div>
          <div>
            <div className="text-xs font-semibold text-white flex items-center gap-1.5">
              <span>商品智能选题匹配</span>
              <span className="text-[10px] text-neutral-400 bg-neutral-800 px-1.5 py-0.5 rounded font-normal">
                按商品版本缓存24h
              </span>
            </div>
            <p className="text-xs text-neutral-400 mt-0.5">
              从同品类前20个候选选题中筛选最匹配视角，提供钩子与推荐理由。
            </p>
          </div>
        </div>

        {/* Product selector dropdown */}
        <div className="flex items-center gap-2 shrink-0">
          <span className="text-xs text-neutral-400 whitespace-nowrap">当前比对商品:</span>
          <select
            value={selectedProduct?.id || ''}
            onChange={(e) => {
              const found = products.find((p) => p.id === e.target.value) || null
              setSelectedProduct(found)
              if (found) setSelectedCategory(found.category)
            }}
            className="text-xs font-medium rounded-xl border border-neutral-700 bg-neutral-800 text-white px-3 py-2 focus:border-amber-400 focus:outline-none"
          >
            {products.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({p.category})
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Category Tabs */}
      <div className="flex items-center gap-1.5 overflow-x-auto pb-1 scrollbar-none">
        <button
          onClick={() => setSelectedCategory('all')}
          className={`px-3 py-1.5 rounded-lg text-xs font-medium whitespace-nowrap transition-all ${
            selectedCategory === 'all'
              ? 'bg-neutral-900 text-white shadow-2xs'
              : 'bg-white text-neutral-600 hover:bg-neutral-100 border border-neutral-200/80'
          }`}
        >
          全部行业榜单 ({topics.length})
        </button>
        {PRODUCT_CATEGORIES.map((cat) => {
          const count = topics.filter((t) => t.category === cat).length
          return (
            <button
              key={cat}
              onClick={() => setSelectedCategory(cat)}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium whitespace-nowrap transition-all ${
                selectedCategory === cat
                  ? 'bg-neutral-900 text-white shadow-2xs'
                  : 'bg-white text-neutral-600 hover:bg-neutral-100 border border-neutral-200/80'
              }`}
            >
              {cat} {count > 0 && <span className="opacity-70 text-[11px]">({count})</span>}
            </button>
          )
        })}
      </div>

      {/* Topics Leaderboard List */}
      <div className="flex flex-col gap-4">
        {displayedTopics.map((topic, index) => {
          const isTop3 = index < 3
          return (
            <div
              key={topic.id}
              className={`group flex flex-col md:flex-row items-stretch md:items-center justify-between gap-5 p-5 rounded-2xl bg-white border transition-all hover:shadow-md ${
                isTop3
                  ? 'border-neutral-300 shadow-xs ring-1 ring-neutral-900/5'
                  : 'border-neutral-200/80'
              }`}
            >
              {/* Left: Rank & Title & Analysis */}
              <div className="flex items-start gap-4 flex-1">
                {/* Rank Number */}
                <div
                  className={`w-9 h-9 rounded-xl flex items-center justify-center font-mono font-bold text-sm shrink-0 mt-0.5 ${
                    index === 0
                      ? 'bg-amber-500 text-white shadow-sm'
                      : index === 1
                      ? 'bg-neutral-800 text-white'
                      : index === 2
                      ? 'bg-neutral-600 text-white'
                      : 'bg-neutral-100 text-neutral-500'
                  }`}
                >
                  {index === 0 ? <Flame className="w-5 h-5 text-white" /> : index + 1}
                </div>

                <div className="flex flex-col gap-2 flex-1">
                  {/* Category, Author, Growth Status */}
                  <div className="flex flex-wrap items-center gap-2 text-xs">
                    <CategoryBadge category={topic.category} />
                    <span className="font-medium text-neutral-700">@{topic.author}</span>
                    <span className="text-neutral-300">·</span>
                    <StatusBadge status={topic.status_tag} />

                    {topic.hourly_growth !== null && (
                      <span className="font-mono text-emerald-600 font-semibold bg-emerald-50 px-2 py-0.5 rounded-full text-[11px]">
                        +{topic.hourly_growth.toLocaleString()} /小时
                      </span>
                    )}

                    {topic.match_score && (
                      <span className="bg-amber-50 text-amber-800 border border-amber-200/80 px-2 py-0.5 rounded-full font-semibold text-[11px] flex items-center gap-1">
                        <Sparkles className="w-3 h-3 text-amber-500" />
                        商品匹配度 {topic.match_score}%
                      </span>
                    )}
                  </div>

                  {/* Title */}
                  <h3 className="text-sm sm:text-base font-bold text-neutral-900 group-hover:text-black leading-snug">
                    {topic.title}
                  </h3>

                  {/* Tags */}
                  <div className="flex flex-wrap gap-1.5">
                    {topic.tags.map((tag, tIdx) => (
                      <span
                        key={tIdx}
                        className="text-[11px] text-neutral-500 bg-neutral-100 px-2 py-0.5 rounded-md"
                      >
                        #{tag}
                      </span>
                    ))}
                  </div>

                  {/* Model Extraction: Angle & Hook */}
                  <div className="mt-1 p-3 rounded-xl bg-neutral-50/80 border border-neutral-200/60 flex flex-col gap-1.5 text-xs">
                    <div className="flex items-start gap-1.5 text-neutral-700">
                      <Lightbulb className="w-3.5 h-3.5 text-amber-500 shrink-0 mt-0.5" />
                      <span className="font-semibold shrink-0">选题视角：</span>
                      <span className="text-neutral-600">{topic.angle_summary}</span>
                    </div>
                    <div className="flex items-start gap-1.5 text-neutral-700">
                      <span className="font-semibold shrink-0 text-purple-700">标题钩子：</span>
                      <span className="italic text-neutral-800">{topic.hook}</span>
                    </div>
                  </div>
                </div>
              </div>

              {/* Right: Metrics & Actions */}
              <div className="flex md:flex-col items-center md:items-end justify-between md:justify-center gap-3 border-t md:border-t-0 md:border-l border-neutral-100 pt-3 md:pt-0 md:pl-5 shrink-0">
                {/* Interaction score pill */}
                <div className="text-left md:text-right">
                  <div className="text-[10px] text-neutral-400 uppercase font-mono tracking-wider">
                    加权互动指数
                  </div>
                  <div className="text-base sm:text-lg font-mono font-bold text-neutral-900">
                    {topic.weighted_interaction.toLocaleString()}
                  </div>
                  {/* Detailed Counts */}
                  <div className="flex items-center gap-2 mt-1 text-[11px] text-neutral-400 font-mono">
                    <span className="flex items-center gap-0.5" title="点赞数">
                      <ThumbsUp className="w-3 h-3 text-neutral-400" />
                      {topic.likes_count?.toLocaleString() ?? '—'}
                    </span>
                    <span className="flex items-center gap-0.5" title="评论数">
                      <MessageCircle className="w-3 h-3 text-neutral-400" />
                      {topic.comments_count?.toLocaleString() ?? '—'}
                    </span>
                    <span className="flex items-center gap-0.5" title="分享数">
                      <Share2 className="w-3 h-3 text-neutral-400" />
                      {topic.shares_count?.toLocaleString() ?? '—'}
                    </span>
                    <span className="flex items-center gap-0.5" title="收藏数">
                      <Bookmark className="w-3 h-3 text-neutral-400" />
                      {topic.collects_count?.toLocaleString() ?? '—'}
                    </span>
                  </div>
                </div>

                {/* Action Buttons */}
                <div className="flex items-center gap-2">
                  <a
                    href={topic.original_url}
                    target="_blank"
                    rel="noreferrer"
                    className="p-2 rounded-xl text-neutral-400 hover:text-neutral-700 hover:bg-neutral-100 transition-colors"
                    title="在抖音查看原文"
                  >
                    <ExternalLink className="w-4 h-4" />
                  </a>

                  <button
                    onClick={() => onSelectTopicForGeneration(topic, selectedProduct || undefined)}
                    className="flex items-center gap-1.5 px-3.5 py-2 rounded-xl bg-neutral-900 text-white hover:bg-black font-semibold text-xs shadow-xs transition-colors"
                  >
                    <Wand2 className="w-3.5 h-3.5 text-amber-400" />
                    带入生成
                  </button>
                </div>
              </div>
            </div>
          )
        })}
      </div>

      {/* Topic Import Modal */}
      <TopicImportModal
        isOpen={importModalOpen}
        onClose={() => setImportModalOpen(false)}
        onImportSuccess={loadData}
      />
    </div>
  )
}
