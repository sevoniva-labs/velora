-- Velora 本地 Compose 初始化（仅首次空数据目录时执行）。
-- 创建两个相互隔离的 database，并创建 Velora 应用账号（最小权限，仅管理 velora 库）。
-- velora: Velora 自身业务数据；casdoor: Casdoor 身份数据（Velora 永不直连）。

CREATE USER velora WITH PASSWORD 'velora';
CREATE DATABASE velora OWNER velora;
CREATE DATABASE casdoor;
