// Los mismos trazos que los mockups de diseno/.

interface Props {
  tamano?: number
  color?: string
  grosor?: number
}

function base({ tamano = 19, color = 'currentColor', grosor = 1.7 }: Props) {
  return {
    width: tamano,
    height: tamano,
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: color,
    strokeWidth: grosor,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
  }
}

export function IconoAntena(p: Props) {
  return (
    <svg {...base(p)}>
      <path d="M12 20v-7" />
      <path d="M8.5 13 12 4l3.5 9" />
      <path d="M4.9 16.5a9 9 0 0 1 0-9" />
      <path d="M19.1 7.5a9 9 0 0 1 0 9" />
    </svg>
  )
}

export function IconoParrilla(p: Props) {
  return (
    <svg {...base(p)}>
      <rect x="3" y="5" width="18" height="16" rx="2" />
      <path d="M3 10h18M8 3v4M16 3v4" />
    </svg>
  )
}

export function IconoReglas(p: Props) {
  return (
    <svg {...base(p)}>
      <rect x="3" y="4" width="18" height="7" rx="2" />
      <rect x="3" y="14" width="18" height="6" rx="2" />
    </svg>
  )
}

export function IconoBiblioteca(p: Props) {
  return (
    <svg {...base(p)}>
      <rect x="3" y="3" width="7" height="7" rx="1.5" />
      <rect x="14" y="3" width="7" height="7" rx="1.5" />
      <rect x="3" y="14" width="7" height="7" rx="1.5" />
      <rect x="14" y="14" width="7" height="7" rx="1.5" />
    </svg>
  )
}

export function IconoAnuncios(p: Props) {
  return (
    <svg {...base(p)}>
      <path d="M20.6 13.4 12 22l-9-9V4h9l8.6 8.6a1.4 1.4 0 0 1 0 2Z" />
      <circle cx="7.5" cy="7.5" r="1.2" />
    </svg>
  )
}

export function IconoSalidas(p: Props) {
  return (
    <svg {...base(p)}>
      <path d="M10 4H5a1 1 0 0 0-1 1v14a1 1 0 0 0 1 1h5" />
      <path d="M14 8l5 4-5 4" />
      <path d="M19 12H9" />
    </svg>
  )
}

export function IconoAjustes(p: Props) {
  return (
    <svg {...base(p)}>
      <circle cx="12" cy="12" r="3.2" />
      <path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.5 1.5M17.6 17.6l1.5 1.5M19.1 4.9l-1.5 1.5M6.4 17.6l-1.5 1.5" />
    </svg>
  )
}

export function IconoMano(p: Props) {
  return (
    <svg {...base(p)}>
      <path d="M9 11V5.5a1.5 1.5 0 0 1 3 0V11" />
      <path d="M12 11V4.5a1.5 1.5 0 0 1 3 0V11" />
      <path d="M15 11V6.5a1.5 1.5 0 0 1 3 0V14a7 7 0 0 1-7 7h-1a6 6 0 0 1-6-6v-3.5a1.5 1.5 0 0 1 3 0" />
    </svg>
  )
}

export function IconoMas(p: Props) {
  return (
    <svg {...base({ grosor: 2.4, ...p })}>
      <path d="M12 5v14M5 12h14" />
    </svg>
  )
}

export function IconoBuscar(p: Props) {
  return (
    <svg {...base(p)}>
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-3.5-3.5" />
    </svg>
  )
}

export function IconoOk(p: Props) {
  return (
    <svg {...base({ grosor: 2.4, ...p })}>
      <path d="m4 12.5 5 5L20 6.5" />
    </svg>
  )
}

export function IconoEquis(p: Props) {
  return (
    <svg {...base({ grosor: 2.4, ...p })}>
      <path d="M6 6l12 12M18 6 6 18" />
    </svg>
  )
}

export function IconoAlerta(p: Props) {
  return (
    <svg {...base(p)}>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7.5v5M12 16.2v.2" />
    </svg>
  )
}

/** Una chincheta: el bloque que alguien movió a mano y quedó clavado. */
export function IconoChincheta(p: Props) {
  return (
    <svg {...base(p)}>
      <path d="M12 17v4" />
      <path d="M9 3h6l-1 5 3 3v2H7v-2l3-3-1-5Z" />
    </svg>
  )
}
