import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { Navigate } from 'react-router'
import { z } from 'zod'
import { ApiError, login } from '../api/auth'
import AuthCard from '../components/AuthCard'
import Button from '../components/Button'
import Field from '../components/Field'
import Notice from '../components/Notice'
import PageMessage from '../components/PageMessage'
import TextLink from '../components/TextLink'
import { currentUserKey, useCurrentUser } from '../hooks/useCurrentUser'

const schema = z.object({
  email: z.email('請輸入有效的 email'),
  password: z.string().min(1, '請輸入密碼'),
})

type LoginValues = z.infer<typeof schema>

function errorMessage(err: Error) {
  if (err instanceof ApiError && err.status === 401) return 'email 或密碼錯誤'
  if (err instanceof ApiError && err.status === 423) return '帳號已鎖定，請聯絡管理員'
  if (err instanceof ApiError && err.status === 429) return '嘗試次數過多，請稍後再試'
  return '登入失敗，請稍後再試'
}

function LoginPage() {
  const me = useCurrentUser()
  const queryClient = useQueryClient()
  const { register, handleSubmit, setValue, formState } = useForm<LoginValues>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', password: '' },
  })
  const mutation = useMutation({
    mutationFn: login,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: currentUserKey }),
    onError: () => setValue('password', ''),
  })

  if (me.isPending) return <PageMessage>載入中…</PageMessage>
  if (me.data) return <Navigate to="/" replace />

  const { errors } = formState
  return (
    <AuthCard title="登入" lead="">
      <form className="flex flex-col gap-4" noValidate onSubmit={handleSubmit((values) => mutation.mutate(values))}>
        {mutation.isError && (
          <Notice tone="error">
            {errorMessage(mutation.error)}
          </Notice>
        )}
        <Field
          id="login-email"
          label="Email"
          type="email"
          autoComplete="email"
          error={errors.email?.message}
          {...register('email')}
        />
        <Field
          id="login-password"
          label="密碼"
          type="password"
          autoComplete="current-password"
          error={errors.password?.message}
          {...register('password')}
        />
        <TextLink className="-mt-2 self-end text-sm" to="/forgot-password">
          忘記密碼？
        </TextLink>
        <Button type="submit" block disabled={mutation.isPending}>
          {mutation.isPending ? '登入中…' : '登入'}
        </Button>
      </form>
      <p className="mt-5 text-center text-sm text-muted-foreground">
        還沒有帳號？
        <TextLink to="/register">
          註冊新帳號
        </TextLink>
      </p>
    </AuthCard>
  )
}

export default LoginPage
