import type { CSSProperties } from 'react'

// Иконки рисуются маской по форме SVG из макета, а цвет берут из currentColor:
// так они перекрашиваются вместе с текстом — в активном пункте меню, на кнопках и в тёмной теме.
export function Icon({ src, size, className }: { src: string; size: number; className?: string }) {
  return <Mask src={src} width={size} height={size} className={className ? `icon ${className}` : 'icon'} />
}

export function Mask({ src, width, height, className }: { src: string; width: number; height: number; className?: string }) {
  const style = { width, height, '--mask': `url("${src}")` } as CSSProperties
  return <span className={className ? `mask ${className}` : 'mask'} style={style} aria-hidden />
}
