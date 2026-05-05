import type { Material } from '../../types'

interface Props {
  materials: Material[]
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + units[i]
}

function getFileIcon(mimeType: string): string {
  if (mimeType.startsWith('image/')) return '🖼️'
  if (mimeType === 'application/pdf') return '📄'
  if (mimeType.startsWith('text/')) return '📝'
  if (mimeType.includes('spreadsheet') || mimeType.includes('excel')) return '📊'
  if (mimeType.includes('presentation') || mimeType.includes('powerpoint')) return '📽️'
  if (mimeType.includes('word') || mimeType.includes('document')) return '📃'
  return '📎'
}

function getFileTypeLabel(mimeType: string): string {
  if (mimeType.startsWith('image/')) {
    const fmt = mimeType.split('/').pop()?.toUpperCase() || ''
    return fmt ? `${fmt} 图片` : '图片'
  }
  if (mimeType === 'application/pdf') return 'PDF'
  if (mimeType.startsWith('text/')) return '文本'
  return mimeType
}

function isImage(mimeType: string): boolean {
  return mimeType.startsWith('image/')
}

function getDataUrl(material: Material): string | null {
  if (!material.content) return null
  if (isImage(material.mime_type)) {
    return `data:${material.mime_type};base64,${material.content}`
  }
  if (material.mime_type === 'application/pdf' || material.mime_type.startsWith('text/')) {
    return `data:${material.mime_type};base64,${material.content}`
  }
  return null
}

function handleDownload(material: Material) {
  if (!material.content) return
  const dataUrl = getDataUrl(material)
  if (!dataUrl) return

  const a = document.createElement('a')
  a.href = dataUrl
  a.download = material.file_name
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

function handlePreview(material: Material) {
  const dataUrl = getDataUrl(material)
  if (!dataUrl) return

  if (isImage(material.mime_type)) {
    window.open(dataUrl, '_blank')
  } else {
    handleDownload(material)
  }
}

export default function MaterialList({ materials }: Props) {
  if (!materials || materials.length === 0) return null

  return (
    <div className="bg-gray-50 border border-gray-200 rounded-xl p-5">
      <h3 className="text-sm font-semibold text-gray-800 mb-3">📎 附件材料 ({materials.length})</h3>
      <div className="space-y-2">
        {materials.map(m => (
          <div
            key={m.id}
            className="flex items-center gap-3 bg-white rounded-lg border border-gray-200 p-3 hover:border-gray-300 transition-colors"
          >
            <span className="text-xl shrink-0">{getFileIcon(m.mime_type)}</span>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-gray-900 truncate" title={m.file_name}>
                {m.file_name}
              </p>
              <p className="text-xs text-gray-400">
                {formatFileSize(m.file_size)} · {getFileTypeLabel(m.mime_type)}
              </p>
            </div>
            <div className="flex items-center gap-1 shrink-0">
              {isImage(m.mime_type) && m.content && (
                <div className="w-10 h-10 rounded border border-gray-200 overflow-hidden shrink-0">
                  <img
                    src={`data:${m.mime_type};base64,${m.content}`}
                    alt={m.file_name}
                    className="w-full h-full object-cover cursor-pointer"
                    onClick={() => handlePreview(m)}
                  />
                </div>
              )}
              <button
                onClick={() => handleDownload(m)}
                className="p-1.5 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
                title="下载"
                disabled={!m.content}
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                </svg>
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
