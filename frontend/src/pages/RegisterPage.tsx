import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { Link, Navigate } from 'react-router'
import { z } from 'zod'
import { ApiError, register as registerUser } from '../api/auth'
import AuthCard from '../components/AuthCard'
import Field from '../components/Field'
import { currentUserKey, useCurrentUser } from '../hooks/useCurrentUser'
import { passwordHint, passwordSchema } from '../validation/password'

const schema = z
  .object({
    email: z.email('請輸入有效的 email'),
    password: passwordSchema,
    confirm_password: z.string(),
  })
  .refine((v) => v.password === v.confirm_password, {
    message: '兩次輸入的密碼不同',
    path: ['confirm_password'],
  })

type RegisterValues = z.infer<typeof schema>

function errorMessage(err: Error) {
  if (err instanceof ApiError && err.status === 409) return '這個 email 已經註冊過'
  if (err instanceof ApiError && err.status === 422) return '資料格式不正確'
  return '註冊失敗，請稍後再試'
}

function RegisterPage() {
  const me = useCurrentUser()
  const queryClient = useQueryClient()
  const { register, handleSubmit, formState } = useForm<RegisterValues>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', password: '', confirm_password: '' },
  })
  const mutation = useMutation({
    mutationFn: registerUser,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: currentUserKey }),
  })

  if (me.isPending) return <p className="container page-message">載入中…</p>
  if (me.data) return <Navigate to="/" replace />

  const { errors } = formState
  return (
    <AuthCard title="註冊" lead="">
      <form className="auth-form" noValidate onSubmit={handleSubmit((values) => mutation.mutate(values))}>
        {mutation.isError && (
          <p className="form-alert" role="alert">
            {errorMessage(mutation.error)}
          </p>
        )}
        <Field
          id="register-email"
          label="Email"
          type="email"
          autoComplete="email"
          error={errors.email?.message}
          {...register('email')}
        />
        <Field
          id="register-password"
          label="密碼"
          type="password"
          autoComplete="new-password"
          hint={passwordHint}
          error={errors.password?.message}
          {...register('password')}
        />
        <Field
          id="register-confirm-password"
          label="確認密碼"
          type="password"
          autoComplete="new-password"
          error={errors.confirm_password?.message}
          {...register('confirm_password')}
        />
        <button type="submit" className="button button-block" disabled={mutation.isPending}>
          {mutation.isPending ? '註冊中…' : '註冊'}
        </button>
      </form>
      <p className="auth-switch">
        已經有帳號？
        <Link className="link" to="/login">
          登入
        </Link>
      </p>
    </AuthCard>
  )
}

export default RegisterPage
