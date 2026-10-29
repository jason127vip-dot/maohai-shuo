const MAX_FILE_BYTES = 10 * 1024 * 1024
const ACCEPTED_TYPES = new Set(['image/jpeg', 'image/png', 'image/webp'])

export interface SelectedPhoto { file: File; previewUrl: string }

function loadImage(url: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image()
    image.onload = () => resolve(image)
    image.onerror = () => reject(new Error('这张图片无法读取，请换一张 JPEG、PNG 或 WebP 照片。'))
    image.src = url
  })
}

function canvasBlob(canvas: HTMLCanvasElement): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(blob => blob ? resolve(blob) : reject(new Error('照片处理失败，请换一张再试。')), 'image/jpeg', 0.88)
  })
}

/** Decode first, then resize locally; no image is uploaded until the user submits. */
export async function preparePhoto(file: File): Promise<SelectedPhoto> {
  if (!ACCEPTED_TYPES.has(file.type)) throw new Error('请选择 JPEG、PNG 或 WebP 图片。')
  if (file.size > MAX_FILE_BYTES) throw new Error('照片超过 10 MB，请选择小一点的照片。')
  const url = URL.createObjectURL(file)
  try {
    const image = await loadImage(url)
    if (image.naturalWidth * image.naturalHeight > 40_000_000) throw new Error('照片分辨率过大，请缩小后上传。')
    if (Math.min(image.naturalWidth, image.naturalHeight) < 128) throw new Error('照片太小了，请选择更清晰的宠物照片。')
    const scale = Math.min(1, 2048 / Math.max(image.naturalWidth, image.naturalHeight))
    const canvas = document.createElement('canvas')
    canvas.width = Math.round(image.naturalWidth * scale)
    canvas.height = Math.round(image.naturalHeight * scale)
    const context = canvas.getContext('2d')
    if (!context) throw new Error('当前浏览器无法处理照片，请在新版微信或浏览器重试。')
    context.fillStyle = '#ffffff'
    context.fillRect(0, 0, canvas.width, canvas.height)
    context.drawImage(image, 0, 0, canvas.width, canvas.height)
    const compressed = await canvasBlob(canvas)
    const processed = new File([compressed], 'pet-photo.jpg', { type: 'image/jpeg' })
    return { file: processed, previewUrl: URL.createObjectURL(processed) }
  } finally { URL.revokeObjectURL(url) }
}

/** Mock-only preview: original image with explicit demo labels, never an AI result. */
export async function createDemoImage(photo: File): Promise<string> {
  const url = URL.createObjectURL(photo)
  try {
    const image = await loadImage(url)
    const canvas = document.createElement('canvas')
    canvas.width = 1024
    canvas.height = 1024
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('图片预览失败，请稍后重试。')
    ctx.fillStyle = '#f7f0e5'
    ctx.fillRect(0, 0, 1024, 1024)
    const scale = Math.min(936 / image.naturalWidth, 820 / image.naturalHeight)
    const width = image.naturalWidth * scale
    const height = image.naturalHeight * scale
    ctx.drawImage(image, (1024 - width) / 2, 40 + (820 - height) / 2, width, height)
    ctx.fillStyle = '#4b3e35'
    ctx.font = 'bold 30px sans-serif'
    ctx.fillText('流程预览 · 原图展示，非 AI 成品', 44, 925)
    ctx.fillStyle = '#ed6946'
    ctx.font = '24px sans-serif'
    ctx.fillText('毛孩儿说 · AI 生成标识示意', 44, 978)
    return canvas.toDataURL('image/jpeg', 0.9)
  } finally { URL.revokeObjectURL(url) }
}
