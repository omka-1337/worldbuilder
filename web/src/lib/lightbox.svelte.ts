// The full-screen picture viewer: the pictures to step through and which
// one is shown. Articles open it with their own pictures; the viewer can
// switch to every picture of the world.

export interface Shot {
  url: string
  caption?: string
}

export const lightbox = $state({
  shots: [] as Shot[],
  index: 0,
  open: false,
})

export function openLightbox(shots: Shot[], index: number) {
  lightbox.shots = shots
  lightbox.index = Math.max(0, Math.min(index, shots.length - 1))
  lightbox.open = shots.length > 0
}

export function closeLightbox() {
  lightbox.open = false
}

// The file name from its URL, for pictures without a caption.
export const fileName = (url: string) => decodeURIComponent(url.split('/').at(-1) ?? '')
