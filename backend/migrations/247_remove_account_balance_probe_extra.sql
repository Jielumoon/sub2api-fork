-- Migration: 247_remove_account_balance_probe_extra
-- 账号级余额探测已移除，余额与额度改由独立的 upstreams 实体负责（见 246_add_upstreams）。
-- 清理 0.2.16-fork.2 写入 accounts.extra 的三个受管键：
--   - upstream_balance_probe_type：探测类型选择
--   - upstream_balance_probe：探测快照
--   - upstream_balance_probe_revision：受管配置版本
-- 只更新真正带这些键的行，避免整表写；重复执行无副作用。
-- 账号倍率探测的键（upstream_billing_probe*、upstream_billing_rate_sync_enabled）不受影响。
UPDATE accounts
SET extra = COALESCE(extra, '{}'::jsonb)
      - 'upstream_balance_probe_type'
      - 'upstream_balance_probe'
      - 'upstream_balance_probe_revision'
WHERE extra ?| ARRAY['upstream_balance_probe_type', 'upstream_balance_probe', 'upstream_balance_probe_revision'];
