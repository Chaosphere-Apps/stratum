import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { BackendAIProviderConfig } from '../src/backendApi'
import { CopilotDraftModal, VisionImportModal } from '../src/components/AIImportModals'

const aiConnection: BackendAIProviderConfig = {
  enabled: true,
  provider: 'OpenAI',
  model: 'gpt-4.1',
  baseUrl: 'https://api.openai.com/v1',
  apiKeySet: true,
}

describe('CopilotDraftModal', () => {
  it('shows configured provider context and captures the design prompt', () => {
    const onClose = vi.fn()
    render(<CopilotDraftModal aiConnection={aiConnection} onClose={onClose} />)
    expect(screen.getByText('OpenAI / gpt-4.1')).toBeTruthy()
    fireEvent.change(screen.getByRole('textbox', { name: 'Design prompt' }), { target: { value: 'Design a payment ledger' } })
    expect((screen.getByRole('textbox', { name: 'Design prompt' }) as HTMLTextAreaElement).value).toBe('Design a payment ledger')
    expect((screen.getByRole('button', { name: 'Generate draft' }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('explains that an AI provider is required', () => {
    render(<CopilotDraftModal aiConnection={null} onClose={vi.fn()} />)
    expect(screen.getByText('Connect an AI provider to enable draft generation.')).toBeTruthy()
  })
})

describe('VisionImportModal', () => {
  beforeEach(() => {
    vi.stubGlobal('URL', {
      ...URL,
      createObjectURL: vi.fn(() => 'blob:whiteboard-preview'),
      revokeObjectURL: vi.fn(),
    })
  })

  afterEach(() => vi.unstubAllGlobals())

  it('previews a selected image, enables generation, and revokes its object URL', async () => {
    const file = new File(['whiteboard'], 'architecture.png', { type: 'image/png' })
    const { unmount } = render(<VisionImportModal aiConnection={aiConnection} onClose={vi.fn()} />)
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    fireEvent.change(input, { target: { files: [file] } })

    await waitFor(() => expect(screen.getByAltText('architecture.png')).toBeTruthy())
    expect(screen.getByText('architecture.png')).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Generate architecture' }) as HTMLButtonElement).disabled).toBe(false)
    unmount()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:whiteboard-preview')
  })

  it('keeps generation disabled without central AI configuration', () => {
    render(<VisionImportModal aiConnection={null} onClose={vi.fn()} />)
    expect(screen.getByText('AI provider required')).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Generate architecture' }) as HTMLButtonElement).disabled).toBe(true)
  })
})
