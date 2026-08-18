/* oxlint-disable react/only-export-components -- 路由配置非组件文件，页面级 lazy 不影响 fast refresh */
import { lazy } from 'react'
import { createBrowserRouter, Navigate } from 'react-router-dom'
import App from './App'
import AdminApp from './AdminApp'
import RequireAuth from './auth/RequireAuth'
import Login from './pages/Login'

// 页面级代码分割。
const Home = lazy(() => import('./pages/Home'))
const Applications = lazy(() => import('./pages/Applications'))
const ApplicationDetail = lazy(() => import('./pages/ApplicationDetail'))
const Favorites = lazy(() => import('./pages/Favorites'))

const AdminDashboard = lazy(() => import('./pages/admin/Dashboard'))
const AdminApplications = lazy(() => import('./pages/admin/Applications'))
const AdminCategories = lazy(() => import('./pages/admin/Categories'))
const AdminTags = lazy(() => import('./pages/admin/Tags'))
const AdminPolicies = lazy(() => import('./pages/admin/Policies'))
const AdminAudit = lazy(() => import('./pages/admin/Audit'))
const AdminSettings = lazy(() => import('./pages/admin/Settings'))

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <Login />,
  },
  {
    path: '/',
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
      { path: '*', element: <Navigate to="/home" replace /> },
    ],
  },
  {
    path: '/admin',
    element: (
      <RequireAuth>
        <AdminApp />
      </RequireAuth>
    ),
    children: [
      { index: true, element: <AdminDashboard /> },
      { path: 'applications', element: <AdminApplications /> },
      { path: 'categories', element: <AdminCategories /> },
      { path: 'tags', element: <AdminTags /> },
      { path: 'policies', element: <AdminPolicies /> },
      { path: 'audit', element: <AdminAudit /> },
      { path: 'settings', element: <AdminSettings /> },
    ],
  },
])
