import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { Link, useLocation } from 'react-router'
import { z } from 'zod'
import { ApiError, resetPassword } from '../api/auth'
import AuthCard from '../components/AuthCard'
import Field from '../components/Field'
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
  return '重設失敗，請稍後再試'
}

function ResetPasswordPage() {
  const token = tokenFromHash(useLocation().hash)
  const queryClient = useQueryClient()
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
        <p className="form-alert" role="alert">
          連結不完整，請重新申請重設密碼。
        </p>
        <p className="auth-switch">
          <Link className="link" to="/forgot-password">
            重新申請
          </Link>
        </p>
      </AuthCard>
    )
  }

  if (mutation.isSuccess) {
    return (
      <AuthCard title="重設密碼" lead="">
        <p className="form-notice" role="status">
          密碼已重設，所有裝置都已登出，請用新密碼登入。
        </p>
        <p className="auth-switch">
          <Link className="link" to="/login">
            前往登入
          </Link>
        </p>
      </AuthCard>
    )
  }

  const { errors } = formState
  return (
    <AuthCard title="重設密碼" lead="">
      <form className="auth-form" noValidate onSubmit={handleSubmit((values) => mutation.mutate(values))}>
        {mutation.isError && (
          <p className="form-alert" role="alert">
            {errorMessage(mutation.error)}
          </p>
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
        <button type="submit" className="button button-block" disabled={mutation.isPending}>
          {mutation.isPending ? '重設中…' : '重設密碼'}
        </button>
      </form>
    </AuthCard>
  )
}

export default ResetPasswordPage
