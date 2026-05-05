import { useState, useRef, useCallback } from 'react'
import { api } from '../../lib/api'
import type { Material } from '../../types'

interface Props {
  sessionId: string
  sessionStatus: string
  materials: Material[]
  onMaterialsChange: (materials: Material[]) => void
}

const MAX_FILE_SIZE = 50 * 1024 * 1024 // 50MB

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
  if (mimeType.startsWith('image/')) return '图片'
  if (mimeType === 'application/pdf') return 'PDF'
  if (mimeType.startsWith('text/')) return '文本'
  return mimeType.split('/').pop()?.toUpperCase() || mimeType
}

export default function MaterialUploader({ sessionId, sessionStatus, materials, onMaterialsChange }: Props) {
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState('')
  const [dragOver, setDragOver] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const isEditable = sessionStatus === 'draft'

  const loadMaterials = useCallback(async () => {
    if (!sessionId) return
    try {
      const data = await api.getSessionMaterials(sessionId)
      onMaterialsChange(data || [])
    } catch {
      // Silently fail on reload
    }
  }, [sessionId, onMaterialsChange])

  const handleFile = async (file: File) => {
    setError('')

    if (file.size > MAX_FILE_SIZE) {
      setError(`文件 "${file.name}" 超过 50MB 限制`)
      return
    }

    setUploading(true)
    try {
      const base64 = await fileToBase64(file)
      const mimeType = file.type || 'application/octet-stream'
      await api.uploadSessionMaterial(sessionId, file.name, mimeType, base64)
      await loadMaterials()
    } catch (err: any) {
      setError(err.message || `上传 "${file.name}" 失败`)
    } finally {
      setUploading(false)
    }
  }

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files
    if (!files || files.length === 0) return
    handleFile(files[0])
    // Reset input so same file can be re-selected
    e.target.value = ''
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setDragOver(false)
    const files = e.dataTransfer.files
    if (files.length > 0) {
      handleFile(files[0])
    }
  }

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault()
    setDragOver(true)
  }

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault()
    setDragOver(false)
  }

  const handleDelete = async (materialId: string) => {
    setError('')
    try {
      await api.deleteSessionMaterial(materialId)
      await loadMaterials()
    } catch (err: any) {
      setError(err.message || '删除失败')
    }
  }

  return (
    <div className="border-t border-gray-200 pt-3 mt-3">
      <h4 className="text-xs font-semibold text-gray-500 uppercase mb-2">会议材料</h4>

      {/* Upload area (only when editable) */}
      {isEditable && (
        <>
          {/* Drop zone */}
          <div
            className={`border-2 border-dashed rounded-lg p-3 text-center cursor-pointer transition-colors mb-2 ${
              dragOver
                ? 'border-blue-400 bg-blue-50'
                : 'border-gray-300 hover:border-gray-400 bg-gray-50'
            }`}
            onDrop={handleDrop}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onClick={() => document.getElementById('material-file-input')?.click()}
          >
            <input
              id="material-file-input"
              type="file"
              className="hidden"
              onChange={handleFileSelect}
              accept="*/*"
            />
            {uploading ? (
              <div className="flex items-center justify-center gap-2">
                <svg className="animate-spin h-4 w-4 text-blue-600" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
                <span className="text-xs text-gray-500">上传中...</span>
              </div>
            ) : (
              <div>
                <svg className="w-6 h-6 mx-auto text-gray-400 mb-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 16v-4m0 0l-2 2m2-2l2 2m-2-8v4m0 0H8m4 0h4M3 12a9 9 0 1118 0 9 9 0 01-18 0z" />
                </svg>
                <p className="text-xs text-gray-500">点击或拖拽文件到此处上传</p>
                <p className="text-[10px] text-gray-400 mt-0.5">支持文本、图片、PDF 等，最大 50MB</p>
              </div>
            )}
          </div>

          {error && (
            <p className="text-xs text-red-500 mb-2">{error}</p>
          )}
        </>
      )}

      {/* File list */}
      {materials.length === 0 ? (
        <p className="text-xs text-gray-400 text-center py-2">暂无材料</p>
      ) : (
        <div className="space-y-1 max-h-48 overflow-y-auto">
          {materials.map(m => (
            <div
              key={m.id}
              className="group flex items-center gap-2 px-2 py-1.5 rounded-lg hover:bg-gray-50 transition-colors"
            >
              <span className="text-base shrink-0">{getFileIcon(m.mime_type)}</span>
              <div className="min-w-0 flex-1">
                <p className="text-xs font-medium text-gray-700 truncate" title={m.file_name}>
                  {m.file_name}
                </p>
                <p className="text-[10px] text-gray-400">
                  {formatFileSize(m.file_size)} · {getFileTypeLabel(m.mime_type)}
                </p>
              </div>
              {isEditable && (
                <button
                  onClick={() => handleDelete(m.id)}
                  className="opacity-0 group-hover:opacity-100 text-gray-400 hover:text-red-500 transition-opacity p-0.5 shrink-0"
                  title="删除此材料"
                >
                  <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = reader.result as string
      // Strip "data:...;base64," prefix to get raw base64
      const idx = result.indexOf(';base64,')
      if (idx >= 0) {
        resolve(result.slice(idx + 8))
      } else {
        resolve(result)
      }
    }
    reader.onerror = () => reject(new Error('读取文件失败'))
    reader.readAsDataURL(file)
  })
}
