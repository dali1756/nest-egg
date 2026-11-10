import Button from './Button'
import Notice from './Notice'

function LoadError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="mt-6 flex flex-col items-start gap-3">
      <Notice tone="error">{message}</Notice>
      <Button type="button" variant="quiet" onClick={onRetry}>
        重新載入
      </Button>
    </div>
  )
}

export default LoadError
