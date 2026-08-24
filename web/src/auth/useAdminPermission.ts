import { hasPermission } from './permissions'
import { useMe } from './useMe'

/** Keep visible page actions aligned with the backend permission boundary. */
export function useAdminPermission(permission: string): boolean {
  const me = useMe()
  return hasPermission(me.data?.permissions, permission, me.data?.roles)
}
