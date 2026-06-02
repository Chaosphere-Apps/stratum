import { useEffect, useState } from 'react'
import { Image as ImageIcon, Upload } from 'lucide-react'

import type { BackendAIProviderConfig } from '../backendApi'
import { TextareaField } from './FormFields'

type AIConnectionMetadata = BackendAIProviderConfig

export function CopilotDraftModal({
  aiConnection,
  onClose,
}: {
  aiConnection: AIConnectionMetadata | null
  onClose: () => void
}) {
  const [prompt, setPrompt] = useState('')
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="ai-settings-modal" role="dialog" aria-modal="true" aria-labelledby="copilot-title">
        <div className="modal-heading">
          <div>
            <p className="eyebrow">Copilot</p>
            <h2 id="copilot-title">Generate a first draft</h2>
            <span>{aiConnection ? `${aiConnection.provider} / ${aiConnection.model}` : 'Connect an AI provider to enable draft generation.'}</span>
          </div>
        </div>
        <TextareaField
          label="Design prompt"
          value={prompt}
          onChange={setPrompt}
        />
        <div className="analysis-empty">
          <strong>Coming next</strong>
          <span>The backend contract will generate structured components, connectors, and requirement assumptions from this prompt.</span>
        </div>
        <div className="modal-actions">
          <button className="secondary-action" type="button" onClick={onClose}>
            Close
          </button>
          <button className="primary-action" type="button" disabled>
            Generate draft
          </button>
        </div>
      </section>
    </div>
  )
}

export function VisionImportModal({
  aiConnection,
  onClose,
}: {
  aiConnection: AIConnectionMetadata | null
  onClose: () => void
}) {
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const canGenerate = Boolean(aiConnection && selectedFile)

  useEffect(() => {
    if (!selectedFile) {
      setPreviewUrl(null)
      return
    }
    const objectUrl = URL.createObjectURL(selectedFile)
    setPreviewUrl(objectUrl)
    return () => URL.revokeObjectURL(objectUrl)
  }, [selectedFile])

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="vision-import-modal" role="dialog" aria-modal="true" aria-labelledby="vision-import-title">
        <div className="modal-heading">
          <div>
            <p className="eyebrow">AI Vision</p>
            <h2 id="vision-import-title">Import a whiteboard sketch</h2>
            <span>
              Upload a whiteboard image so Stratum can later extract components, flows, and assumptions into a structured design.
            </span>
          </div>
        </div>

        <label className={`vision-dropzone ${previewUrl ? 'has-preview' : ''}`}>
          <input
            type="file"
            accept="image/png,image/jpeg,image/webp,image/gif"
            onChange={(event) => setSelectedFile(event.target.files?.[0] ?? null)}
          />
          {previewUrl ? (
            <img src={previewUrl} alt={selectedFile?.name ?? 'Selected whiteboard'} />
          ) : (
            <div>
              <Upload size={24} />
              <strong>Choose whiteboard image</strong>
              <span>PNG, JPG, WebP, or GIF</span>
            </div>
          )}
        </label>

        {selectedFile ? (
          <div className="vision-file-summary">
            <ImageIcon size={18} />
            <div>
              <strong>{selectedFile.name}</strong>
              <span>{Math.max(1, Math.round(selectedFile.size / 1024))} KB</span>
            </div>
          </div>
        ) : null}

        <div className="analysis-empty">
          <strong>{aiConnection ? 'Vision pipeline placeholder' : 'AI provider required'}</strong>
          <span>
            {aiConnection
              ? 'Next backend step: send this image to the configured vision model and return structured components and connectors for review.'
              : 'Configure an AI provider before generating architecture from images.'}
          </span>
        </div>

        <div className="modal-actions">
          <button className="secondary-action" type="button" onClick={onClose}>
            Close
          </button>
          <button className="primary-action" type="button" disabled={!canGenerate}>
            Generate architecture
          </button>
        </div>
      </section>
    </div>
  )
}
