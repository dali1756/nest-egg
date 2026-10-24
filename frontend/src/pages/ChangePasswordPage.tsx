import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { ApiError, changePassword } from '../api/auth'
import AuthCard from '../components/AuthCard'
import Button from '../components/Button'
import Field from '../components/Field'
import Notice from '../components/Notice'
import { clearUserIfUnauthorized } from '../hooks/useCurrentUser'
import { passwordHint, passwordSchema } from '../validation/password'

const schema = z
  .object({
    current_password: z.string().min(1, '請輸入目前的密碼'),
    password: passwordSchema,
    confirm_password: z.string(),
  })
  .refine((v) => v.password === v.confirm_password, {
    message: '兩次輸入的密碼不同',
    path: ['confirm_password'],
  })

type ChangeValues = z.infer<typeof schema>

function errorMessage(err: Error) {
  if (err instanceof ApiError && err.status === 403) return '目前的密碼不正確'
  if (err instanceof ApiError && err.status === 423) return '帳號已鎖定，請聯絡管理員'
  if (err instanceof ApiError && err.status === 422) return '資料格式不正確'
  if (err instanceof ApiError && err.status === 429) return '嘗試次數過多，請稍後再試'
  return '變更失敗，請稍後再試'
}

function ChangePasswordPage() {
  const queryClient = useQueryClient()
  const { register, handleSubmit, formState, reset, setValue } = useForm<ChangeValues>({
    resolver: zodResolver(schema),
    defaultValues: { current_password: '', password: '', confirm_password: '' },
  })
  const mutation = useMutation({
    mutationFn: changePassword,
    onSuccess: () => reset(),
    onError: (err) => {
      setValue('current_password', '')
      clearUserIfUnauthorized(queryClient, err)
    },
  })

  const { errors } = formState
  return (
    <AuthCard title="變更密碼" lead="">
      <form className="flex flex-col gap-4" noValidate onSubmit={handleSubmit((values) => mutation.mutate(values))}>
        {mutation.isSuccess && (
          <Notice tone="success">
            密碼已變更，其他裝置都已登出。
          </Notice>
        )}
        {mutation.isError && (
          <Notice tone="error">
            {errorMessage(mutation.error)}
          </Notice>
        )}
        <Field
          id="change-current-password"
          label="目前的密碼"
          type="password"
          autoComplete="current-password"
          error={errors.current_password?.message}
          {...register('current_password')}
        />
        <Field
          id="change-password"
          label="新密碼"
          type="password"
          autoComplete="new-password"
          hint={passwordHint}
          error={errors.password?.message}
          {...register('password')}
        />
        <Field
          id="change-confirm-password"
          label="確認新密碼"
          type="password"
          autoComplete="new-password"
          error={errors.confirm_password?.message}
          {...register('confirm_password')}
        />
        <Button type="submit" block disabled={mutation.isPending}>
          {mutation.isPending ? '變更中...' : '變更密碼'}
        </Button>
      </form>
    </AuthCard>
  )
}

export default ChangePasswordPage
