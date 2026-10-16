import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useLocation, useNavigate } from 'react-router'
import { z } from 'zod'
import { ApiError, resetPassword } from '../api/auth'
import AuthCard from '../components/AuthCard'
import Button from '../components/Button'
import Field from '../components/Field'
import Notice from '../components/Notice'
import TextLink from '../components/TextLink'
import { currentUserKey } from '../hooks/useCurrentUser'
import { passwordHint, passwordSchema } from '../validation/password'

const schema = z
  .object({
    password: passwordSchema,
    confirm_password: z.string(),
  })
  .refine((v) => v.password === v.confirm_password, {
    message: '兩次輸入的密碼不同',
    path: ['confirm_password'],
  })

type ResetValues = z.infer<typeof schema>

function tokenFromHash(hash: string) {
  return new URLSearchParams(hash.replace(/^#/, '')).get('token') ?? ''
}

function errorMessage(err: Error) {
  if (err instanceof ApiError && err.status === 400) return '連結無效或已過期，請重新申請'
  if (err instanceof ApiError && err.status === 422) return '資料格式不正確'
  if (err instanceof ApiError && err.status === 429) return '嘗試次數過多，請稍後再試'
  return '重設失敗，請稍後再試'
}

function ResetPasswordPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const [token] = useState(() => tokenFromHash(location.hash))
  const queryClient = useQueryClient()

  useEffect(() => {
    if (location.hash) navigate({ hash: '' }, { replace: true })
  }, [location.hash, navigate])

  const { register, handleSubmit, formState } = useForm<ResetValues>({
    resolver: zodResolver(schema),
    defaultValues: { password: '', confirm_password: '' },
  })
  const mutation = useMutation({
    mutationFn: (values: ResetValues) => resetPassword({ token, ...values }),
    onSuccess: () => queryClient.setQueryData(currentUserKey, null),
  })

  if (!token) {
    return (
      <AuthCard title="重設密碼" lead="">
        <Notice tone="error">
          連結不完整，請重新申請重設密碼。
        </Notice>
        <p className="mt-5 text-center text-sm text-muted-foreground">
          <TextLink to="/forgot-password">
            重新申請
          </TextLink>
        </p>
      </AuthCard>
    )
  }

  if (mutation.isSuccess) {
    return (
      <AuthCard title="重設密碼" lead="">
        <Notice tone="success">
          密碼已重設，所有裝置都已登出，請用新密碼登入。
        </Notice>
        <p className="mt-5 text-center text-sm text-muted-foreground">
          <TextLink to="/login">
            前往登入
          </TextLink>
        </p>
      </AuthCard>
    )
  }

  const { errors } = formState
  return (
    <AuthCard title="重設密碼" lead="">
      <form className="flex flex-col gap-4" noValidate onSubmit={handleSubmit((values) => mutation.mutate(values))}>
        {mutation.isError && (
          <Notice tone="error">
            {errorMessage(mutation.error)}
          </Notice>
        )}
        <Field
          id="reset-password"
          label="新密碼"
          type="password"
          autoComplete="new-password"
          hint={passwordHint}
          error={errors.password?.message}
          {...register('password')}
        />
        <Field
          id="reset-confirm-password"
          label="確認新密碼"
          type="password"
          autoComplete="new-password"
          error={errors.confirm_password?.message}
          {...register('confirm_password')}
        />
        <Button type="submit" block disabled={mutation.isPending}>
          {mutation.isPending ? '重設中…' : '重設密碼'}
        </Button>
      </form>
    </AuthCard>
  )
}

export default ResetPasswordPage
