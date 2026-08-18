-- Velora 本地 Compose 初始化：创建两个相互隔离的 database。
-- velora: Velora 自身业务数据；casdoor: Casdoor 身份数据（Velora 永不直连）。
CREATE DATABASE velora;
CREATE DATABASE casdoor;
