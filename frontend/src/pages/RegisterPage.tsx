import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { Navigate } from 'react-router'
import { z } from 'zod'
import { ApiError, register as registerUser } from '../api/auth'
import AuthCard from '../components/AuthCard'
import Button from '../components/Button'
import Field from '../components/Field'
import Notice from '../components/Notice'
import PageMessage from '../components/PageMessage'
import TextLink from '../components/TextLink'
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
  if (err instanceof ApiError && err.status === 429) return '嘗試次數過多，請稍後再試'
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

  if (me.isPending) return <PageMessage>載入中…</PageMessage>
  if (me.data) return <Navigate to="/" replace />

  const { errors } = formState
  return (
    <AuthCard title="註冊" lead="">
      <form className="flex flex-col gap-4" noValidate onSubmit={handleSubmit((values) => mutation.mutate(values))}>
        {mutation.isError && (
          <Notice tone="error">
            {errorMessage(mutation.error)}
          </Notice>
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
        <Button type="submit" block disabled={mutation.isPending}>
          {mutation.isPending ? '註冊中…' : '註冊'}
        </Button>
      </form>
      <p className="mt-5 text-center text-sm text-muted-foreground">
        已經有帳號？
        <TextLink to="/login">
          登入
        </TextLink>
      </p>
    </AuthCard>
  )
}

export default RegisterPage
