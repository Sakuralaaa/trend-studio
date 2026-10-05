package studio

import (
 "encoding/json"
 "time"
)

var Categories = []string{"服装鞋包","美妆个护","家居日用","食品饮料","数码家电","母婴","运动户外","其他"}
const Showcase = "product_showcase"
const Douyin = "douyin_sales"
const PromptVersion = "2026-10-v1"
const TemplateVersion = "2026-10-v1"

type User struct { ID string `json:"id"`; TenantID *string `json:"tenant_id"`; Email string `json:"email"`; Name string `json:"name"`; Role string `json:"role"` }
type Tenant struct { ID string `json:"id"`; Name string `json:"name"`; Balance int64 `json:"balance"`; Reserved int64 `json:"reserved_balance"` }
type Reference struct { ID string `json:"id"`; URL string `json:"url"`; FileName string `json:"file_name"`; FileSize int64 `json:"file_size"`; Position int `json:"sort_order"`; Primary bool `json:"is_primary"`; Width int `json:"width"`; Height int `json:"height"` }
type Product struct {
 ID string `json:"id"`; TenantID string `json:"tenant_id"`; Name string `json:"name"`; Category string `json:"category"`
 Brand string `json:"brand"`; SKU string `json:"external_sku"`; Color string `json:"color"`; Price *float64 `json:"price"`
 Points []string `json:"selling_points"`; Scenarios []string `json:"usage_scenarios"`
 References []Reference `json:"reference_images"`; Status string `json:"status"`; Version int `json:"version"`
 CreatedAt time.Time `json:"created_at"`; UpdatedAt time.Time `json:"updated_at"`
}
type CopyData struct {
 Titles []string `json:"titles"`; Description string `json:"description"`; Points []string `json:"selling_points"`
 Script *Script `json:"script,omitempty"`; Caption *Caption `json:"caption,omitempty"`
}
type Script struct { Text string `json:"text"`; Seconds int `json:"estimated_seconds"` }
type Caption struct { Text string `json:"text"`; Tags []string `json:"tags"` }
type Pricing struct { Text int64 `json:"text"`; Image int64 `json:"image"`; Version int `json:"version"` }
type Limits struct { Image int `json:"image_concurrency"`; Text int `json:"text_concurrency"`; Collect int `json:"collection_concurrency"`; Background int `json:"background_daily_calls"` }
type Provider struct {
 ID string `json:"id"`; Kind string `json:"kind"`; Name string `json:"name"`; BaseURL string `json:"base_url"`; Model string `json:"model"`
 APIKey string `json:"api_key,omitempty"`; Configured bool `json:"configured"`
 ImageField string `json:"image_field"`; MaxReferences int `json:"max_references"`; Size string `json:"size"`
 TimeoutSeconds int `json:"timeout_seconds"`; VisionModel string `json:"vision_model,omitempty"`
 ResultHosts []string `json:"result_hosts"`; InputRate float64 `json:"input_rate_per_million"`; OutputRate float64 `json:"output_rate_per_million"`; Currency string `json:"currency"`
}
type CollectorConfig struct { BaseURL string `json:"base_url"`; APIKey string `json:"api_key,omitempty"`; Configured bool `json:"configured"` }
type Counters struct { Likes *int64 `json:"likes_count"`; Comments *int64 `json:"comments_count"`; Shares *int64 `json:"shares_count"`; Collects *int64 `json:"collects_count"` }
type Topic struct {
 ID string `json:"id"`; Author string `json:"author"`; Title string `json:"title"`; Description string `json:"description"`; Tags []string `json:"tags"`
 URL string `json:"original_url"`; PublishedAt *time.Time `json:"published_at"`; FetchedAt time.Time `json:"crawled_at"`; Counters
 Interaction *float64 `json:"weighted_interaction"`; Growth *float64 `json:"hourly_growth"`; Score float64 `json:"trend_score"`
 StatusTag string `json:"status_tag"`; Angle string `json:"angle_summary"`; Hook string `json:"hook"`
 Category string `json:"category"`; MatchScore *int `json:"match_score,omitempty"`; MatchReason string `json:"match_reason,omitempty"`
 Imported bool `json:"is_imported_by_user"`
}
type Source struct { ID string `json:"id"`; Name string `json:"account_name"`; URL string `json:"url"`; AccountID string `json:"account_id"`; Category string `json:"category"`; Active bool `json:"active"`; LastSuccess *time.Time `json:"last_crawled_at"`; LastError *string `json:"last_error"` }
type QuoteItem struct { Kind string `json:"item_type"`; Name string `json:"name"`; Credits int64 `json:"credits"`; ProviderType string `json:"provider_type"` }
type Snapshot struct { Product Product `json:"product"`; Topic *Topic `json:"topic,omitempty"`; TextProviderID string `json:"text_provider_id"`; ImageProviderID string `json:"image_provider_id"`; Pricing Pricing `json:"pricing"`; PromptVersion string `json:"prompt_version"`; TemplateVersion string `json:"template_version"` }
type Quote struct {
 ID string `json:"quote_id"`; ProductID string `json:"product_id"`; ProductName string `json:"product_name"`; Preset string `json:"preset"`; TopicID string `json:"topic_id,omitempty"`
 Items []QuoteItem `json:"items"`; Total int64 `json:"total_credits"`; ExpiresAt time.Time `json:"expires_at"`; IdempotencyKey string `json:"idempotency_key"`
 Snapshot Snapshot `json:"-"`; GenerationID string `json:"generation_id,omitempty"`; RetryKind string `json:"retry_kind,omitempty"`
}
type StoredQuote struct { Quote Quote `json:"quote"`; Snapshot Snapshot `json:"snapshot"` }
type AssetMeta struct { FileName string `json:"file_name"`; FileSize int64 `json:"file_size"`; Width int `json:"width"`; Height int `json:"height"`; CopyVersion int `json:"copy_version"`; Label string `json:"label"` }
type Asset struct { ID string `json:"id"`; URL string `json:"url"`; Kind string `json:"kind"`; AssetMeta; Selected bool `json:"is_selected"`; CreatedAt time.Time `json:"created_at"` }
type Item struct { ID string `json:"id"`; Kind string `json:"type"`; Name string `json:"name"`; Status string `json:"status"`; Credits int64 `json:"credits"`; Error *string `json:"error_message"`; Content string `json:"content,omitempty"`; Versions []Asset `json:"versions"`; ProviderType string `json:"provider_type"`; Width int `json:"width"`; Height int `json:"height"` }
type Generation struct {
 ID string `json:"id"`; ProductID string `json:"product_id"`; ProductName string `json:"product_name"`; ProductImageURL string `json:"product_image_url"`; ProductCategory string `json:"product_category"`
 Preset string `json:"preset"`; Status string `json:"status"`; TopicTitle string `json:"topic_title,omitempty"`
 Reserved int64 `json:"reserved_credits"`; Settled int64 `json:"settled_credits"`; Released int64 `json:"released_credits"`
 Copy CopyData `json:"copy_data"`; CopyVersion int `json:"copy_version"`; RenderPending bool `json:"render_pending"`
 Items []Item `json:"artifacts"`; Snapshot Snapshot `json:"input_snapshot"`; CreatedAt time.Time `json:"created_at"`; UpdatedAt time.Time `json:"updated_at"`
}
type Job struct { ID string; Kind string; TenantID *string; GenerationID *string; Data json.RawMessage; Attempts int }
type APIError struct { Status int; Code string; Message string }
func (e *APIError) Error() string { return e.Message }
func problem(status int, code, message string) error { return &APIError{status,code,message} }
func assetURL(id string) string { return "/api/v1/assets/"+id }
func nameFor(kind string) string { return map[string]string{"main_image":"商品主图","scene_image":"商品场景图","selling_point_image":"商品卖点图","vertical_cover":"竖版封面","copywriting":"商品文案"}[kind] }
func providerFor(kind string) string { if kind=="copywriting" { return "text" }; if kind=="selling_point_image"||kind=="vertical_cover" { return "template" }; return "image" }
func planItems(preset string, p Pricing) []QuoteItem {
 items:=[]QuoteItem{{"copywriting",nameFor("copywriting"),p.Text,"text"}}
 if preset==Showcase { items=append(items,QuoteItem{"main_image",nameFor("main_image"),p.Image,"image"}) }
 items=append(items,QuoteItem{"scene_image",nameFor("scene_image"),p.Image,"image"})
 kind:="selling_point_image"; if preset==Douyin { kind="vertical_cover" }
 return append(items,QuoteItem{kind,nameFor(kind),0,"template"})
}
