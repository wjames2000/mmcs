-- MMCS 管理员账号种子数据
-- 密码: admin123
-- 导入方式: psql -U postgres -d mmcs -f seed_admin.sql

INSERT INTO public.users (id, name, email, password_hash, avatar_url, status, created_at, updated_at)
VALUES (
  'u_admin_001',
  '管理员',
  'admin@mmcs.local',
  '$2a$10$426aRYtdsZEpVW.lutd1eeujOqDWGmhhl55H5s1HpDmIgpm9k6/d2',
  NULL,
  'active',
  NOW(),
  NOW()
)
ON CONFLICT (email) DO UPDATE
SET name = '管理员', updated_at = NOW();
