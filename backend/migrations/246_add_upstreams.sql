-- Migration: 246_add_upstreams
-- 上游实体：余额/额度的真实归属是「站点地址 + 查询用 API Key」，不是本地账号。
-- 多个账号可能指向同一上游，挂在账号 extra 上会重复探测同一份余额。
--
-- 表结构说明：
--   - api_key_encrypted 列存放 AES-256-GCM 密文（base64），由 service 层加密，
--     与 channel_monitors.api_key_encrypted 同一套 SecretEncryptor。
--   - config_version 用于拒绝迟到的探测结果：地址 / Key / 代理变更即换新值，
--     在途探测写回时比较该列，避免旧配置的结果覆盖新配置。
--   - probe 存放探测快照（JSONB）；next_probe_at / last_attempt_at /
--     failure_count / last_error 单独成列，因为到期扫描要走索引。
--   - (enabled, next_probe_at) 索引服务调度器扫描「到期需要探测」的上游。

CREATE TABLE IF NOT EXISTS upstreams (
    id                 BIGSERIAL PRIMARY KEY,
    name               VARCHAR(100) NOT NULL,
    base_url           VARCHAR(500) NOT NULL,
    api_key_encrypted  TEXT         NOT NULL,
    proxy_id           BIGINT       REFERENCES proxies(id) ON DELETE SET NULL,
    enabled            BOOLEAN      NOT NULL DEFAULT TRUE,
    interval_minutes   INT          NOT NULL DEFAULT 15,
    config_version     VARCHAR(64)  NOT NULL,
    probe              JSONB,
    next_probe_at      TIMESTAMPTZ,
    last_attempt_at    TIMESTAMPTZ,
    failure_count      INT          NOT NULL DEFAULT 0,
    last_error         VARCHAR(100) NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    CONSTRAINT upstreams_interval_check CHECK (interval_minutes BETWEEN 5 AND 1440)
);

CREATE INDEX IF NOT EXISTS idx_upstreams_deleted_at
    ON upstreams (deleted_at);
CREATE INDEX IF NOT EXISTS idx_upstreams_proxy_id
    ON upstreams (proxy_id);
CREATE INDEX IF NOT EXISTS idx_upstreams_enabled_next_probe_at
    ON upstreams (enabled, next_probe_at)
    WHERE deleted_at IS NULL;
