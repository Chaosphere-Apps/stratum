import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { CanvasErrorBoundary } from '../src/components/CanvasErrorBoundary'
import { StatelessModeBanner } from '../src/components/StatelessModeBanner'

function BrokenCanvas(): never {
  throw new Error('invalid snapshot')
}

describe('status and failure surfaces', () => {
  it('shows stateless risk only when applicable', () => {
    const { rerender } = render(<StatelessModeBanner storageStatus={null} />)
    expect(screen.queryByRole('status')).toBeNull()
    rerender(<StatelessModeBanner storageStatus={{ stateless: true, warning: 'Restart loses data' } as never} />)
    expect(screen.getByRole('status').textContent).toContain('Restart loses data')
  })

  it('recovers the canvas through the explicit reset action', async () => {
    const user = userEvent.setup()
    const reset = vi.fn()
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    render(<CanvasErrorBoundary onResetCanvas={reset}><BrokenCanvas /></CanvasErrorBoundary>)
    expect(screen.getByText('invalid snapshot')).toBeTruthy()
    await user.click(screen.getByRole('button', { name: 'Reset canvas data' }))
    expect(reset).toHaveBeenCalledOnce()
  })
})
