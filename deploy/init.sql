-- 在 1Panel → 数据库 → MySQL 中执行（或创建库/用户时参考）
CREATE DATABASE IF NOT EXISTS blog DEFAULT CHARSET utf8mb4;
CREATE USER IF NOT EXISTS 'blog'@'%' IDENTIFIED BY '改成强密码';
GRANT ALL PRIVILEGES ON blog.* TO 'blog'@'%';
FLUSH PRIVILEGES;
-- 表结构由 blog-stats 启动时自动创建（page_views_daily）
