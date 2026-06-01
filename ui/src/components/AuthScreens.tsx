import { useState } from 'react'
import { ShieldCheck } from 'lucide-react'
import { Field } from './FormFields'

export function FirstAdminOnboarding({
  error,
  isSaving,
  onCreate,
}: {
  error: string | null
  isSaving: boolean
  onCreate: (input: { displayName: string; email: string; password: string }) => void
}) {
  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  return (
    <main className="onboarding-shell">
      <section className="onboarding-card">
        <div className="home-logo">S</div>
        <p className="eyebrow">First run setup</p>
        <h1>Create the first admin</h1>
        <p>Stratum has no users yet. The first account becomes the organization admin and can configure SSO and user access.</p>
        {error ? <div className="home-error inline-error"><span>{error}</span></div> : null}
        <Field label="Name" value={displayName} onChange={setDisplayName} />
        <Field label="Work email" value={email} onChange={setEmail} />
        <Field label="Password" type="password" value={password} onChange={setPassword} />
        <button
          className="primary-action full-width"
          type="button"
          disabled={isSaving || !displayName.trim() || !email.trim() || password.length < 8}
          onClick={() => onCreate({ displayName, email, password })}
        >
          <ShieldCheck size={17} /> {isSaving ? 'Creating admin...' : 'Create admin'}
        </button>
      </section>
    </main>
  )
}

export function LoginScreen({
  requiresPasswordSetup,
  error,
  isSaving,
  onLogin,
  onSetInitialPassword,
}: {
  requiresPasswordSetup: boolean
  error: string | null
  isSaving: boolean
  onLogin: (input: { email: string; password: string }) => void
  onSetInitialPassword: (input: { email: string; password: string }) => void
}) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const title = requiresPasswordSetup ? 'Set admin password' : 'Sign in to Stratum'
  const description = requiresPasswordSetup
    ? 'Existing users were created before local passwords existed. Set the first admin password to continue.'
    : 'Use your workspace account to access designs, reviews, and admin settings.'
  return (
    <main className="onboarding-shell">
      <section className="onboarding-card">
        <div className="home-logo">S</div>
        <p className="eyebrow">Enterprise access</p>
        <h1>{title}</h1>
        <p>{description}</p>
        {error ? <div className="home-error inline-error"><span>{error}</span></div> : null}
        <Field label="Work email" value={email} onChange={setEmail} />
        <Field label="Password" type="password" value={password} onChange={setPassword} />
        <button
          className="primary-action full-width"
          type="button"
          disabled={isSaving || !email.trim() || password.length < 8}
          onClick={() => (requiresPasswordSetup ? onSetInitialPassword({ email, password }) : onLogin({ email, password }))}
        >
          <ShieldCheck size={17} /> {isSaving ? 'Checking...' : requiresPasswordSetup ? 'Set password and continue' : 'Sign in'}
        </button>
      </section>
    </main>
  )
}
