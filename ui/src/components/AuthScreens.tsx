import { useState } from 'react'
import { Building2, ShieldCheck } from 'lucide-react'
import type { PasswordPolicy, PublicAuthConfig } from '../backendApi'
import { Field } from './FormFields'

function passwordMeetsPolicy(password: string, policy: PasswordPolicy) {
  return Array.from(password.trim()).length >= policy.minimumLength
}

function PasswordPolicyHint({ policy }: { policy: PasswordPolicy }) {
  return <small className="field-hint">Use at least {policy.minimumLength} characters.</small>
}

export function FirstAdminOnboarding({
  error,
  isSaving,
  passwordPolicy,
  onCreate,
}: {
  error: string | null
  isSaving: boolean
  passwordPolicy: PasswordPolicy
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
        <PasswordPolicyHint policy={passwordPolicy} />
        <button
          className="primary-action full-width"
          type="button"
          disabled={isSaving || !displayName.trim() || !email.trim() || !passwordMeetsPolicy(password, passwordPolicy)}
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
	authConfig,
}: {
  requiresPasswordSetup: boolean
  error: string | null
  isSaving: boolean
  onLogin: (input: { email: string; password: string }) => void
  onSetInitialPassword: (input: { email: string; password: string }) => void
	authConfig: PublicAuthConfig
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
        {authConfig.ssoEnabled && !requiresPasswordSetup ? (
          <a className="primary-action full-width auth-provider-action" href={authConfig.ssoStartUrl}>
            <Building2 size={17} /> Continue with {authConfig.provider === 'okta' ? 'Okta' : 'SSO'}
          </a>
        ) : null}
        {authConfig.localPasswordEnabled || requiresPasswordSetup ? (
          <>
            {authConfig.ssoEnabled && !requiresPasswordSetup ? <div className="auth-divider"><span>or use a local account</span></div> : null}
            <Field label="Work email" value={email} onChange={setEmail} />
            <Field label="Password" type="password" value={password} onChange={setPassword} />
            {requiresPasswordSetup ? <PasswordPolicyHint policy={authConfig.passwordPolicy} /> : null}
            <button
              className="secondary-action full-width"
              type="button"
              disabled={isSaving || !email.trim() || (requiresPasswordSetup ? !passwordMeetsPolicy(password, authConfig.passwordPolicy) : !password)}
              onClick={() => (requiresPasswordSetup ? onSetInitialPassword({ email, password }) : onLogin({ email, password }))}
            >
              <ShieldCheck size={17} /> {isSaving ? 'Checking...' : requiresPasswordSetup ? 'Set password and continue' : 'Sign in locally'}
            </button>
          </>
        ) : null}
      </section>
    </main>
  )
}

export function ResetPasswordScreen({
  token,
  error,
  isSaving,
  isComplete,
  passwordPolicy,
  onReset,
  onSignIn,
}: {
  token: string
  error: string | null
  isSaving: boolean
  isComplete: boolean
  passwordPolicy: PasswordPolicy
  onReset: (input: { token: string; password: string }) => void
  onSignIn: () => void
}) {
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const mismatch = password.length > 0 && confirmPassword.length > 0 && password !== confirmPassword
  const canSubmit = token.trim() && passwordMeetsPolicy(password, passwordPolicy) && password === confirmPassword

  return (
    <main className="onboarding-shell">
      <section className="onboarding-card">
        <div className="home-logo">S</div>
        <p className="eyebrow">Account recovery</p>
        <h1>{isComplete ? 'Password updated' : 'Reset your password'}</h1>
        <p>
          {isComplete
            ? 'Your local Stratum password has been updated. You can now sign in with the new password.'
            : 'Enter a new local password for your Stratum account. Reset links are one-time use and expire automatically.'}
        </p>
        {error ? <div className="home-error inline-error"><span>{error}</span></div> : null}
        {!token.trim() && !isComplete ? <div className="home-error inline-error"><span>This reset link is missing a token.</span></div> : null}
        {isComplete ? (
          <button className="primary-action full-width" type="button" onClick={onSignIn}>
            Sign in
          </button>
        ) : (
          <>
            <Field label="New password" type="password" value={password} onChange={setPassword} />
            <PasswordPolicyHint policy={passwordPolicy} />
            <Field label="Confirm password" type="password" value={confirmPassword} onChange={setConfirmPassword} />
            {mismatch ? <small className="field-hint danger">Passwords do not match.</small> : null}
            <button
              className="primary-action full-width"
              type="button"
              disabled={isSaving || !canSubmit}
              onClick={() => onReset({ token, password })}
            >
              <ShieldCheck size={17} /> {isSaving ? 'Updating...' : 'Reset password'}
            </button>
          </>
        )}
      </section>
    </main>
  )
}
