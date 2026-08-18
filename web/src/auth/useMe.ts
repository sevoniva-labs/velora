import { useQuery } from '@tanstack/react-query'
import { getMe, queryKeys } from '../api/api'

/** 当前用户（GET /me）。401 由 RequireAuth 处理跳转。 */
export function useMe() {
  return useQuery({
    queryKey: queryKeys.me,
    queryFn: getMe,
    retry: false,
    staleTime: 60_000,
  })
}
