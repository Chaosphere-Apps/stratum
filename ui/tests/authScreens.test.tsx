import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { FirstAdminOnboarding, LoginScreen, ResetPasswordScreen } from '../src/components/AuthScreens'

describe('authentication screens', () => {
  const passwordPolicy = { minimumLength: 12 }

  it('validates and submits first-admin setup', async () => {
    const user = userEvent.setup()
    const onCreate = vi.fn()
    render(<FirstAdminOnboarding error={null} isSaving={false} passwordPolicy={passwordPolicy} onCreate={onCreate} />)
    const submit = screen.getByRole('button', { name: 'Create admin' }) as HTMLButtonElement
    expect(submit.disabled).toBe(true)
    await user.type(screen.getByLabelText('Name'), 'Ada Admin')
    await user.type(screen.getByLabelText('Work email'), 'ada@example.test')
    expect(screen.getByText('Use at least 12 characters.')).toBeTruthy()
    await user.type(screen.getByLabelText('Password'), 'secure12')
    expect(submit.disabled).toBe(true)
    await user.type(screen.getByLabelText('Password'), '-password')
    expect(submit.disabled).toBe(false)
    await user.click(submit)
    expect(onCreate).toHaveBeenCalledWith({ displayName: 'Ada Admin', email: 'ada@example.test', password: 'secure12-password' })
  })

  it('routes login and initial-password setup to different actions', async () => {
    const user = userEvent.setup()
    const onLogin = vi.fn()
    const onSetInitialPassword = vi.fn()
    const { rerender } = render(
      <LoginScreen requiresPasswordSetup={false} error="Denied" isSaving={false} onLogin={onLogin} onSetInitialPassword={onSetInitialPassword} authConfig={{ localPasswordEnabled: true, ssoEnabled: false, provider: '', ssoStartUrl: '', passwordPolicy }} />,
    )
    expect(screen.getByText('Denied')).toBeTruthy()
    await user.type(screen.getByLabelText('Work email'), 'member@example.test')
    await user.type(screen.getByLabelText('Password'), 'long-password')
    await user.click(screen.getByRole('button', { name: 'Sign in locally' }))
    expect(onLogin).toHaveBeenCalledOnce()

    rerender(<LoginScreen requiresPasswordSetup error={null} isSaving={false} onLogin={onLogin} onSetInitialPassword={onSetInitialPassword} authConfig={{ localPasswordEnabled: true, ssoEnabled: false, provider: '', ssoStartUrl: '', passwordPolicy }} />)
    await user.type(screen.getByLabelText('Work email'), 'admin@example.test')
    await user.type(screen.getByLabelText('Password'), 'admin-password')
    await user.click(screen.getByRole('button', { name: 'Set password and continue' }))
    expect(onSetInitialPassword).toHaveBeenCalledOnce()
  })

  it('guards password reset and exposes completion', async () => {
    const user = userEvent.setup()
    const onReset = vi.fn()
    const onSignIn = vi.fn()
    const { rerender } = render(
      <ResetPasswordScreen token="reset-token" error={null} isSaving={false} isComplete={false} passwordPolicy={passwordPolicy} onReset={onReset} onSignIn={onSignIn} />,
    )
    await user.type(screen.getByLabelText('New password'), 'new-password')
    await user.type(screen.getByLabelText('Confirm password'), 'different-password')
    expect(screen.getByText('Passwords do not match.')).toBeTruthy()
    const reset = screen.getByRole('button', { name: 'Reset password' }) as HTMLButtonElement
    expect(reset.disabled).toBe(true)
    await user.clear(screen.getByLabelText('Confirm password'))
    await user.type(screen.getByLabelText('Confirm password'), 'new-password')
    await user.click(reset)
    expect(onReset).toHaveBeenCalledWith({ token: 'reset-token', password: 'new-password' })

    rerender(<ResetPasswordScreen token="" error={null} isSaving={false} isComplete passwordPolicy={passwordPolicy} onReset={onReset} onSignIn={onSignIn} />)
    await user.click(screen.getByRole('button', { name: 'Sign in' }))
    expect(onSignIn).toHaveBeenCalledOnce()
  })
})
