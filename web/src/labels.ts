// Velora 界面统一中文标签映射（枚举直出统一走这里）。

import type { SSOType, AppStatus, HealthStatus, PolicyType } from './types'

export const SSO_TYPE_LABEL: Record<SSOType, string> = {
  URL: '直链',
  OIDC: 'OIDC',
  SAML: 'SAML',
  CAS: 'CAS',
  FORWARD_AUTH: '反向代理',
}

export const SSO_TYPE_COLOR: Record<SSOType, string> = {
  URL: 'default',
  OIDC: 'blue',
  SAML: 'purple',
  CAS: 'cyan',
  FORWARD_AUTH: 'orange',
}

export const APP_STATUS_LABEL: Record<AppStatus, string> = {
  ENABLED: '启用',
  DISABLED: '停用',
}

export const APP_STATUS_COLOR: Record<AppStatus, string> = {
  ENABLED: 'success',
  DISABLED: 'default',
}

export const HEALTH_LABEL: Record<HealthStatus, string> = {
  UP: '正常',
  DOWN: '异常',
  UNKNOWN: '未知',
}

export const HEALTH_COLOR: Record<HealthStatus, string> = {
  UP: 'success',
  DOWN: 'error',
  UNKNOWN: 'default',
}

export const POLICY_TYPE_LABEL: Record<PolicyType, string> = {
  EVERYONE: '所有人',
  ORGANIZATION: '指定组织',
  ROLE: '指定角色',
  GROUP: '指定用户组',
  USER: '指定用户',
}

export const AUDIT_ACTION_LABEL: Record<string, string> = {
  LOGIN: '登录',
  LOGOUT: '退出登录',
  APPLICATION_CREATE: '创建应用',
  APPLICATION_UPDATE: '更新应用',
  APPLICATION_DELETE: '删除应用',
  APPLICATION_LAUNCH: '启动应用',
  FAVORITE_ADD: '收藏应用',
  FAVORITE_REMOVE: '取消收藏',
  PERMISSION_CHANGE: '访问策略变更',
  CATEGORY_CREATE: '创建分类',
  CATEGORY_UPDATE: '更新分类',
  CATEGORY_DELETE: '删除分类',
  TAG_CREATE: '创建标签',
  TAG_UPDATE: '更新标签',
  TAG_DELETE: '删除标签',
  SETTING_UPDATE: '更新门户设置',
}

export function auditActionLabel(action: string): string {
  return AUDIT_ACTION_LABEL[action] ?? action
}
