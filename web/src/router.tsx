/* oxlint-disable react/only-export-components -- 路由配置非组件文件，页面级 lazy 不影响 fast refresh */
import { lazy, type ComponentType } from 'react'
import { createBrowserRouter, Navigate } from 'react-router-dom'
import { RouteErrorFallback } from './components/AppErrorBoundary'
import App from './App'
import AdminApp from './AdminApp'
import RequireAuth from './auth/RequireAuth'
import Login from './pages/Login'
import OIDCCallback from './pages/OIDCCallback'
import NotFound from './pages/NotFound'

/**
 * 懒加载容错：部署新版本后，旧页面留存的浏览器标签会请求已不存在的 chunk。
 * 捕获动态导入失败并强制整页刷新一次（拿最新 index.html），避免用户看到错误页；
 * 刷新后仍失败才交给 RouteErrorFallback。
 */
const CHUNK_RELOAD_FLAG = 'velora_chunk_reload'

function lazyWithReload<T extends ComponentType<unknown>>(factory: () => Promise<{ default: T }>) {
  return lazy(async (): Promise<{ default: T }> => {
    try {
      const module = await factory()
      sessionStorage.removeItem(CHUNK_RELOAD_FLAG)
      return module
    } catch (err) {
      if (!sessionStorage.getItem(CHUNK_RELOAD_FLAG)) {
        sessionStorage.setItem(CHUNK_RELOAD_FLAG, '1')
        window.location.reload()
        // 页面即将刷新，挂起当前加载，避免闪现错误页
        return new Promise<{ default: T }>(() => {})
      }
      sessionStorage.removeItem(CHUNK_RELOAD_FLAG)
      throw err
    }
  })
}

// 页面级代码分割。
const Home = lazyWithReload(() => import('./pages/Home'))
const Applications = lazyWithReload(() => import('./pages/Applications'))
const ApplicationDetail = lazyWithReload(() => import('./pages/ApplicationDetail'))
const Favorites = lazyWithReload(() => import('./pages/Favorites'))
const UserCenter = lazyWithReload(() => import('./pages/UserCenter'))

const AdminDashboard = lazyWithReload(() => import('./pages/admin/Dashboard'))
const AdminApplications = lazyWithReload(() => import('./pages/admin/Applications'))
const AdminApplicationManagement = lazyWithReload(() => import('./pages/admin/ApplicationManagement'))
const AdminTaxonomy = lazyWithReload(() => import('./pages/admin/Taxonomy'))
const AdminAudit = lazyWithReload(() => import('./pages/admin/Audit'))
const AdminIntegrationTokens = lazyWithReload(() => import('./pages/admin/IntegrationTokens'))
const AdminUsers = lazyWithReload(() => import('./pages/admin/Users'))
const AdminUserDetail = lazyWithReload(() => import('./pages/admin/UserDetail'))
const AdminOrganization = lazyWithReload(() => import('./pages/admin/Organization'))
const AdminUserGroups = lazyWithReload(() => import('./pages/admin/UserGroups'))
const AdminRoles = lazyWithReload(() => import('./pages/admin/Roles'))
const AdminSessions = lazyWithReload(() => import('./pages/admin/Sessions'))
const AdminApprovals = lazyWithReload(() => import('./pages/admin/Approvals'))
const AdminTemporaryGrants = lazyWithReload(() => import('./pages/admin/TemporaryGrants'))
const AdminAccessReviews = lazyWithReload(() => import('./pages/admin/AccessReviews'))
const AdminConfigChanges = lazyWithReload(() => import('./pages/admin/ConfigChanges'))
const AdminLoginSecurity = lazyWithReload(() => import('./pages/admin/LoginSecurity'))

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <Login />,
    errorElement: <RouteErrorFallback />,
  },
  {
    path: '/auth/callback',
    element: <OIDCCallback />,
    errorElement: <RouteErrorFallback />,
  },
  {
    path: '/',
    errorElement: <RouteErrorFallback />,
    element: (
      <RequireAuth>
        <App />
      </RequireAuth>
    ),
    children: [
      { index: true, element: <Navigate to="/home" replace /> },
      { path: 'home', element: <Home /> },
      { path: 'applications', element: <Applications /> },
      { path: 'applications/:id', element: <ApplicationDetail /> },
      { path: 'favorites', element: <Favorites /> },
      { path: 'user-center', element: <UserCenter /> },
      { path: '*', element: <NotFound /> },
    ],
  },
  {
    path: '/admin',
    errorElement: <RouteErrorFallback />,
    element: (
      <RequireAuth>
        <AdminApp />
      </RequireAuth>
    ),
    children: [
      { index: true, element: <AdminDashboard /> },
      { path: 'applications', element: <AdminApplications /> },
      { path: 'applications/:id', element: <AdminApplicationManagement /> },
      { path: 'taxonomy', element: <AdminTaxonomy /> },
      { path: 'categories', element: <Navigate to="/admin/taxonomy" replace /> },
      { path: 'tags', element: <Navigate to="/admin/taxonomy" replace /> },
      { path: 'policies', element: <Navigate to="/admin/applications" replace /> },
      { path: 'audit', element: <AdminAudit /> },
      { path: 'integration-tokens', element: <AdminIntegrationTokens /> },
      { path: 'identity', element: <Navigate to="/admin/applications" replace /> },
      { path: 'users', element: <AdminUsers /> },
      { path: 'users/:id', element: <AdminUserDetail /> },
      { path: 'organization', element: <AdminOrganization /> },
      { path: 'user-groups', element: <AdminUserGroups /> },
      { path: 'roles', element: <AdminRoles /> },
      { path: 'sessions', element: <AdminSessions /> },
      { path: 'approvals', element: <AdminApprovals /> },
      { path: 'temporary-grants', element: <AdminTemporaryGrants /> },
      { path: 'access-reviews', element: <AdminAccessReviews /> },
      { path: 'config-changes', element: <AdminConfigChanges /> },
      { path: 'login-security', element: <AdminLoginSecurity /> },
      { path: '*', element: <NotFound /> },
    ],
  },
  // 登录后未知路径兜底（不含外壳时也展示 404）。
  { path: '*', element: <NotFound /> },
])
