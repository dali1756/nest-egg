import Button from './Button'

function SaveButton({ pending }: { pending: boolean }) {
  return (
    <Button type="submit" block disabled={pending}>
      {pending ? '儲存中...' : '儲存'}
    </Button>
  )
}

export default SaveButton
