import { Component, type ErrorInfo, type ReactNode } from 'react'

export class CanvasErrorBoundary extends Component<
  { children: ReactNode; onResetCanvas: () => void },
  { error: Error | null }
> {
  state: { error: Error | null } = { error: null }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Stratum canvas error', error, info)
  }

  render() {
    if (!this.state.error) return this.props.children
    return (
      <div className="canvas-error-panel">
        <div>
          <strong>Canvas could not be opened</strong>
          <span>{this.state.error.message || 'The saved canvas data is invalid or from an unsupported version.'}</span>
        </div>
        <div className="canvas-error-actions">
          <button
            type="button"
            onClick={() => {
              this.props.onResetCanvas()
              this.setState({ error: null })
            }}
          >
            Reset canvas data
          </button>
          <button type="button" onClick={() => window.location.reload()}>Refresh</button>
        </div>
      </div>
    )
  }
}
