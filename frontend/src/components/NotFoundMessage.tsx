import TextLink from './TextLink'

type NotFoundMessageProps = { message: string; backTo: string; backLabel: string }

function NotFoundMessage({ message, backTo, backLabel }: NotFoundMessageProps) {
  return (
    <div className="mx-auto mt-12 flex max-w-170 flex-col items-start gap-2 px-5 text-muted-foreground">
      <p>{message}</p>
      <TextLink to={backTo} className="ml-0">
        {backLabel}
      </TextLink>
    </div>
  )
}

export default NotFoundMessage
