package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Upstream holds the schema definition for the Upstream entity.
//
// 上游是余额探测的真实归属：站点地址 + 查询用 API Key。多个账号可能指向同
// 一个上游，因此余额快照挂在实体行上而不是 accounts.extra。
type Upstream struct {
	ent.Schema
}

func (Upstream) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "upstreams"},
	}
}

func (Upstream) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (Upstream) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			MaxLen(100).
			NotEmpty(),
		field.String("base_url").
			MaxLen(500).
			NotEmpty().
			Comment("站点根地址，探测时拼 /v1/usage。"),
		field.String("api_key_encrypted").
			NotEmpty().
			Sensitive().
			Comment("AES-256-GCM 加密的查询用 API Key（base64）。"),
		field.Int64("proxy_id").
			Optional().
			Nillable().
			Comment("可选代理；代理失效时置 NULL。"),
		field.Bool("enabled").
			Default(true),
		field.Int("interval_minutes").
			Default(15).
			Comment("定时探测周期，5–1440 分钟。"),
		field.String("config_version").
			MaxLen(64).
			NotEmpty().
			Comment("配置版本；地址/Key/代理变更即换新值，用于拒绝迟到的探测结果。"),
		field.JSON("probe", map[string]any{}).
			Optional().
			SchemaType(map[string]string{"postgres": "jsonb"}).
			Comment("最近一次探测快照，结构见 service.upstreamProbeSnapshot。"),
		field.Time("next_probe_at").
			Optional().
			Nillable().
			Comment("下一次定时探测时间；NULL 表示立即到期。"),
		field.Time("last_attempt_at").
			Optional().
			Nillable(),
		field.Int("failure_count").
			Default(0),
		field.String("last_error").
			MaxLen(100).
			Default("").
			Comment("受控错误码，不存上游正文。"),
	}
}

func (Upstream) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("deleted_at"),
		index.Fields("proxy_id"),
		index.Fields("enabled", "next_probe_at"),
	}
}
