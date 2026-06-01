import { Database } from 'lucide-react'
import type { BackendStorageStatus } from '../backendApi'

export function StatelessModeBanner({ storageStatus }: { storageStatus: BackendStorageStatus | null }) {
  if (!storageStatus?.stateless) return null
  return (
    <section className="stateless-banner" role="status" aria-live="polite">
      <Database size={16} />
      <div>
        <strong>Stateless mode</strong>
        <span>{storageStatus.warning || 'Data is stored in process cache and will be lost when the backend restarts.'}</span>
      </div>
    </section>
  )
}
