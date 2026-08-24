// Velora 界面统一中文标签映射（枚举直出统一走这里）。

import type { SSOType, AppStatus, HealthStatus, PolicyType } from './types'

export const SSO_TYPE_LABEL: Record<SSOType, string> = {
  URL: '普通链接',
  OIDC: '统一登录',
  VELORA_OIDC: '待迁移：Velora OIDC',
  SAML: 'SAML',
  CAS: 'CAS',
  FORWARD_AUTH: '反向代理',
}

export const SSO_TYPE_COLOR: Record<SSOType, string> = {
  URL: 'default',
  OIDC: 'blue',
  VELORA_OIDC: 'warning',
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

export const APP_LIFECYCLE_LABEL: Record<string, string> = {
  DRAFT: '草稿',
  IDENTITY_PENDING: '待配置登录',
  VERIFICATION_PENDING: '待验证',
  READY: '待上线',
  PUBLISHED: '已上线',
  DISABLED: '已停用',
}

export const APP_LIFECYCLE_COLOR: Record<string, string> = {
  DRAFT: 'default',
  IDENTITY_PENDING: 'processing',
  VERIFICATION_PENDING: 'warning',
  READY: 'blue',
  PUBLISHED: 'success',
  DISABLED: 'default',
}

export const ONBOARDING_STATUS_LABEL: Record<string, string> = {
  DRAFT: '配置中',
  APPROVAL_PENDING: '等待审批',
  CREDENTIALS_ISSUED: '配置已生成',
  WAITING_FOR_DEPLOYMENT: '等待应用部署',
  VERIFIED: '验证通过',
  PILOT: '试运行',
  READY: '待上线',
  PUBLISHED: '已上线',
  ACTION_REQUIRED: '需要处理',
  DEGRADED: '运行异常',
  SUSPENDED: '已停用',
}

export const ONBOARDING_OPERATION_LABEL: Record<string, string> = {
  RECONCILE_PROVIDER: '同步登录配置',
  UPSERT_IDENTITY_BINDING: '更新登录配置',
  VERIFY_IDENTITY: '验证统一登录',
  RUN_CHECKS: '运行上线检查',
  SUBMIT_PUBLISH: '提交上线',
  PUBLISH: '应用上线',
  DISABLE: '停用应用',
}

export const OPERATION_STATUS_LABEL: Record<string, string> = {
  PENDING: '等待执行',
  RUNNING: '执行中',
  SUCCEEDED: '已完成',
  FAILED: '失败',
}

export const ONBOARDING_CHECK_LABEL: Record<string, string> = {
  access_policy: '访问范围',
  oidc_discovery: '统一登录配置',
  provisioning_challenge: '账号下发连接',
  provisioning_duplicate: '重复下发保护',
  provisioning_stale: '过期数据保护',
}

export function enumLabel(labels: Record<string, string>, value?: string, fallback = '未知'): string {
  if (!value) return fallback
  return labels[value] ?? fallback
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
  'auth.login': '登录',
  'auth.logout': '退出登录',
  'auth.password.change': '修改密码',
  'portal.application.create': '创建应用',
  'portal.application.update': '更新应用',
  'portal.application.delete': '删除应用',
  'portal.application.publish': '应用上线',
  'portal.application.submit_publish': '提交应用上线',
  'portal.application.roles.replace': '更新应用角色',
  'portal.application.access_grants.replace': '更新应用使用范围',
  'portal.application.provisioning.upsert': '更新账号下发设置',
  'portal.application.provisioning.retry': '重新下发应用账号',
  'portal.application.credential_approval.create': '申请接入凭据',
  'portal.policy.replace': '更新应用使用范围',
  'iam.integration.update': '更新统一登录配置',
  'iam.console.open': '打开身份引擎应急控制台',
  'auth.federated.login': '统一身份登录',
  'approval.decide': '处理审批',
  'approval.create': '提交审批',
  'role.create': '创建平台角色',
  'role.update': '更新平台角色',
  'role.copy': '复制平台角色',
  'role.permissions.update': '更新角色权限',
  'role.data_scope.update': '更新角色可管理范围',
  'temporary_role_grant.create': '创建临时授权',
  'temporary_role_grant.revoke': '撤销临时授权',
  'access_review.create': '发起权限复核',
  'access_review.item.decide': '提交权限复核结果',
  'security.api_token.create': '创建对接账号',
  'security.api_token.revoke': '停用对接账号',
  'config_change.create': '创建配置变更',
  'config_change.approve': '审批配置变更',
  'config_change.publish': '应用配置变更',
  'config_change.rollback.request': '申请恢复上一版本',
  'config_change.rollback': '恢复上一版本',
  'portal.application.launch': '启动应用',
  'portal.favorite.add': '收藏应用',
  'portal.favorite.remove': '取消收藏',
  'portal.category.create': '创建分类',
  'portal.category.update': '更新分类',
  'portal.category.delete': '删除分类',
  'portal.tag.create': '创建标签',
  'portal.tag.update': '更新标签',
  'portal.tag.delete': '删除标签',
  LOGIN: '登录',
  LOGOUT: '退出登录',
  APPLICATION_CREATE: '创建应用',
  APPLICATION_UPDATE: '更新应用',
  APPLICATION_DELETE: '删除应用',
  APPLICATION_LAUNCH: '启动应用',
  FAVORITE_ADD: '收藏应用',
  FAVORITE_REMOVE: '取消收藏',
  PERMISSION_CHANGE: '应用使用范围变更',
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

export const AUDIT_RESOURCE_LABEL: Record<string, string> = {
  user: '用户', role: '平台角色', session: '登录会话', api_token: '对接账号',
  portal_application: '应用', portal_category: '应用分类', portal_tag: '应用标签',
  application_access: '应用使用范围', identity_integration: '统一登录设置',
  approval: '审批', temporary_role_grant: '临时授权', access_review: '权限复核',
  config_change: '配置变更', identity_console: '身份管理应急入口',
}

export function auditResourceLabel(resource: string): string {
  return AUDIT_RESOURCE_LABEL[resource] ?? '其他对象'
}

export const APPROVAL_TYPE_LABEL: Record<string, string> = {
  APPLICATION_ACCESS_CHANGE: '应用使用范围变更',
  TEMPORARY_ROLE_GRANT: '临时授权',
  USER_ROLE_CHANGE: '用户角色变更',
  ROLE_PERMISSION_CHANGE: '角色权限变更',
  ROLE_DATA_SCOPE_CHANGE: '角色可管理范围变更',
  CONFIG_CHANGE_APPROVE: '配置变更审批',
  CONFIG_CHANGE_PUBLISH: '配置生效',
  CONFIG_CHANGE_ROLLBACK_REQUEST: '恢复上一版本申请',
  CONFIG_CHANGE_ROLLBACK: '恢复上一版本',
  SECURITY_POLICY_CHANGE: '安全策略变更',
}

export function approvalTypeLabel(type: string): string { return APPROVAL_TYPE_LABEL[type] ?? '权限变更' }
