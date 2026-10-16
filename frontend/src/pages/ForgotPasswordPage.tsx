import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { Link } from 'react-router'
import { z } from 'zod'
import { ApiError, forgotPassword } from '../api/auth'
import AuthCard from '../components/AuthCard'
import Field from '../components/Field'

const schema = z.object({
  email: z.email('請輸入有效的 email'),
})

type ForgotValues = z.infer<typeof schema>

function errorMessage(err: Error) {
  if (err instanceof ApiError && err.status === 503) return '忘記密碼功能尚未開放，請聯絡管理員'
  if (err instanceof ApiError && err.status === 429) return '嘗試次數過多，請稍後再試'
  return '送出失敗，請稍後再試'
}

function ForgotPasswordPage() {
  const { register, handleSubmit, formState } = useForm<ForgotValues>({
    resolver: zodResolver(schema),
    defaultValues: { email: '' },
  })
  const mutation = useMutation({
    mutationFn: (values: ForgotValues) => forgotPassword(values.email),
  })

  const { errors } = formState
  return (
    <AuthCard title="忘記密碼" lead="輸入註冊時的 email，我們會寄出重設密碼的連結。">
      {mutation.isSuccess ? (
        <p className="form-notice" role="status">
          如果這個 email 有註冊，重設密碼的連結已經寄出，請在 30 分鐘內使用。
        </p>
      ) : (
        <form className="auth-form" noValidate onSubmit={handleSubmit((values) => mutation.mutate(values))}>
          {mutation.isError && (
            <p className="form-alert" role="alert">
              {errorMessage(mutation.error)}
            </p>
          )}
          <Field
            id="forgot-email"
            label="Email"
            type="email"
            autoComplete="email"
            error={errors.email?.message}
            {...register('email')}
          />
          <button type="submit" className="button button-block" disabled={mutation.isPending}>
            {mutation.isPending ? '送出中…' : '寄出重設連結'}
          </button>
        </form>
      )}
      <p className="auth-switch">
        <Link className="link" to="/login">
          回到登入
        </Link>
      </p>
    </AuthCard>
  )
}

export default ForgotPasswordPage
