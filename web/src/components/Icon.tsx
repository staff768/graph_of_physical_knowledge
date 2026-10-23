export function Icon({ src, size, className }: { src: string; size: number; className?: string }) {
  return <img src={src} width={size} height={size} alt="" aria-hidden className={className ? `icon ${className}` : 'icon'} />
}
