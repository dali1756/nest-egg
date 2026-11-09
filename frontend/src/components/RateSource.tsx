function RateSource({ className = '' }: { className?: string }) {
  return <p className={`text-xs text-destructive-text ${className}`}>資料來源：中央銀行</p>
}

export default RateSource
