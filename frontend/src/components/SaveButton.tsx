import SubmitButton from './SubmitButton'

function SaveButton({ pending }: { pending: boolean }) {
  return (
    <SubmitButton pending={pending} pendingLabel="儲存中...">
      儲存
    </SubmitButton>
  )
}

export default SaveButton
