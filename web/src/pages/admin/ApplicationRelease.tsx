import { App, Button, List, Space, Tag, Typography } from 'antd'
import { ProCard, ProDescriptions } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getApplicationOnboarding, publishApplication, queryKeys, runApplicationOnboardingChecks, submitApplicationPublish, verifyApplicationIdentity } from '../../api/api'
import type { Application } from '../../types'
import QueryErrorState from '../../components/QueryErrorState'
import { APP_LIFECYCLE_LABEL, ONBOARDING_CHECK_LABEL, enumLabel } from '../../labels'
import { formatDateTime } from '../../utils/format'

interface Props { application: Application; canVerify: boolean; canPublish: boolean }

export default function ApplicationRelease({ application, canVerify, canPublish }: Props) {
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const onboarding = useQuery({ queryKey: queryKeys.applicationOnboarding(application.id), queryFn: () => getApplicationOnboarding(application.id) })
  const refresh = async () => { await Promise.all([queryClient.invalidateQueries({ queryKey: queryKeys.applicationOnboarding(application.id) }), queryClient.invalidateQueries({ queryKey: ['admin', 'applications'] })]) }
  const verify = useMutation({ mutationFn: () => verifyApplicationIdentity(application.id, onboarding.data?.binding?.configVersion), onSuccess: async (result) => { message[result.passed ? 'success' : 'error'](result.passed ? '登录验证通过' : '登录验证失败'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '登录验证失败') })
  const checks = useMutation({ mutationFn: () => runApplicationOnboardingChecks(application.id, onboarding.data?.application.configVersion ?? application.configVersion ?? 0), onSuccess: async (result) => { message[result.passed ? 'success' : 'warning'](result.passed ? '发布检查通过' : '发布检查未通过'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '发布检查失败') })
  const submit = useMutation({ mutationFn: () => submitApplicationPublish(application.id, onboarding.data?.application.configVersion), onSuccess: async () => { message.success('应用已进入待发布状态'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '提交发布失败') })
  const publish = useMutation({ mutationFn: () => publishApplication(application.id, onboarding.data?.application.configVersion), onSuccess: async () => { message.success('应用已发布'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '应用发布失败') })
  if (onboarding.isError) return <QueryErrorState refetch={() => void onboarding.refetch()} />
  const data = onboarding.data
  const identityVerified = application.ssoType === 'URL' || data?.binding?.verificationStatus === 'PASSED'
  const allChecksPassed = Boolean(data?.onboardingChecks.length) && data!.onboardingChecks.every((item) => item.result === 'PASSED')
  let action = { label: '运行发布检查', run: () => checks.mutate(), loading: checks.isPending, disabled: !canVerify }
  if (!identityVerified) action = { label: '验证登录配置', run: () => verify.mutate(), loading: verify.isPending, disabled: !canVerify || !data?.binding }
  else if (allChecksPassed && application.lifecycleStatus !== 'READY' && application.lifecycleStatus !== 'PUBLISHED') action = { label: '提交发布', run: () => submit.mutate(), loading: submit.isPending, disabled: !canPublish || !data?.canPublish }
  else if (application.lifecycleStatus === 'READY') action = { label: '发布应用', run: () => publish.mutate(), loading: publish.isPending, disabled: !canPublish || !data?.canPublish }
  return <Space direction="vertical" size={16} style={{ width: '100%' }}>
    <ProDescriptions column={2} loading={onboarding.isLoading} dataSource={data ?? {}} columns={[
      { title: '接入状态', render: () => enumLabel(APP_LIFECYCLE_LABEL, application.lifecycleStatus, '草稿') },
      { title: '最近检查', render: () => formatDateTime(data?.onboardingChecks[0]?.occurredAt) },
      { title: '检查结果', render: () => allChecksPassed ? <Tag color="success">通过</Tag> : <Tag>待检查</Tag> },
      { title: '发布条件', render: () => data?.canPublish ? <Tag color="success">已满足</Tag> : <Tag>未满足</Tag> },
    ]} />
    <ProCard title="发布检查" extra={<Button type="primary" loading={action.loading} disabled={action.disabled} onClick={action.run}>{action.label}</Button>}>
      <List dataSource={data?.onboardingChecks ?? []} locale={{ emptyText: '尚未运行检查' }} renderItem={(item) => <List.Item><Space><Tag color={item.result === 'PASSED' ? 'success' : 'error'}>{item.result === 'PASSED' ? '通过' : '未通过'}</Tag><Typography.Text>{enumLabel(ONBOARDING_CHECK_LABEL, item.checkType, '接入检查')}</Typography.Text>{item.errorCode && <Typography.Text type="secondary">请修复后重试</Typography.Text>}</Space></List.Item>} />
    </ProCard>
  </Space>
}
