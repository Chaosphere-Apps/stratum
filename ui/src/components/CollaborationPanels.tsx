import { useState } from 'react'
import { CheckCircle2, Eye, MessageSquare, Send, UserPlus } from 'lucide-react'
import type {
  BackendDesignComment,
  BackendDesignReviewRequest,
  BackendDesignVersion,
  BackendNotification,
  BackendUser,
} from '../backendApi'
import type { DesignComponent, DesignConnector } from '../types'
import { versionStatusLabel } from '../app/versionLifecycle'
import { roleLabel, userDisplayName } from '../app/format'
import { TextareaField } from './FormFields'

function reviewStatusLabel(status: BackendDesignReviewRequest['status']) {
  if (status === 'changes_requested') return 'Changes requested'
  if (status === 'approved') return 'Approved'
  return 'Requested'
}

export function ReviewWorkspace({
  mode,
  users,
  activeUserId,
  comments,
  reviews,
  activeVersion,
  selectedComponent,
  selectedConnector,
  commentDraft,
  reviewSummaryDrafts,
  isSaving,
  error,
  offline,
  onCommentDraftChange,
  onSubmitComment,
  onRequestReview,
  onReviewSummaryChange,
  onCompleteReview,
  onRefresh,
}: {
  mode: 'view' | 'comment'
  users: BackendUser[]
  activeUserId: string
  comments: BackendDesignComment[]
  reviews: BackendDesignReviewRequest[]
  activeVersion: BackendDesignVersion | null
  selectedComponent: DesignComponent | null
  selectedConnector: DesignConnector | null
  commentDraft: string
  reviewSummaryDrafts: Record<string, string>
  isSaving: boolean
  error: string | null
  offline: boolean
  onCommentDraftChange: (value: string) => void
  onSubmitComment: () => void
  onRequestReview: () => void
  onReviewSummaryChange: (reviewId: string, summary: string) => void
  onCompleteReview: (reviewId: string, status: BackendDesignReviewRequest['status']) => void
  onRefresh: () => void
}) {
  const selectedTarget = selectedComponent?.name || (selectedConnector ? 'selected connection' : 'whole design')
  const activeVersionReviews = activeVersion ? reviews.filter((review) => review.versionId === activeVersion.id) : []
  const approvedReviewCount = activeVersionReviews.filter((review) => review.status === 'approved').length
  const changeRequestCount = activeVersionReviews.filter((review) => review.status === 'changes_requested').length

  return (
    <div className="review-workspace">
      <div className="inspector-heading">
        {mode === 'comment' ? <MessageSquare size={18} /> : <Eye size={18} />}
        <div>
          <strong>{mode === 'comment' ? 'Comment mode' : 'View mode'}</strong>
          <span>Canvas is locked while reviews are captured</span>
        </div>
      </div>

      {offline ? (
        <div className="offline-notice">
          <strong>Collaboration backend offline</strong>
          <span>View mode still works. Start the backend to load or capture comments and review requests.</span>
        </div>
      ) : null}
      {error ? <div className="analysis-error">{error}</div> : null}

      <div className="review-actions">
        <button className="primary-action full-width" type="button" onClick={onRequestReview} disabled={isSaving || offline}>
          <UserPlus size={16} /> Request review
        </button>
        <button className="secondary-action compact-action" type="button" onClick={onRefresh} disabled={isSaving}>
          Refresh
        </button>
      </div>

      {activeVersion ? (
        <section className="review-card version-review-card">
          <div className="section-title">Current version</div>
          <div className="version-review-summary">
            <strong>v{activeVersion.versionNumber}</strong>
            <span className={`version-status-badge ${activeVersion.status}`}>{versionStatusLabel(activeVersion.status)}</span>
          </div>
          <div className="version-review-metrics">
            <span>{activeVersionReviews.length} requested</span>
            <span>{approvedReviewCount} approved</span>
            <span>{changeRequestCount} changes requested</span>
          </div>
        </section>
      ) : null}

      {mode === 'comment' ? (
        <section className="review-card">
          <div className="section-title">Add comment</div>
          <span className="comment-target">Target: {selectedTarget}</span>
          <textarea
            value={commentDraft}
            rows={4}
            placeholder="Capture a question, decision, or review note..."
            onChange={(event) => onCommentDraftChange(event.target.value)}
          />
          <button className="primary-action full-width" type="button" onClick={onSubmitComment} disabled={isSaving || offline || !commentDraft.trim()}>
            <Send size={16} /> Add comment
          </button>
        </section>
      ) : null}

      <section className="review-card">
        <div className="section-title">Review history</div>
        {reviews.length ? (
          <div className="activity-list">
            {[...reviews].sort((left, right) => {
              if (activeVersion && left.versionId === activeVersion.id && right.versionId !== activeVersion.id) return -1
              if (activeVersion && left.versionId !== activeVersion.id && right.versionId === activeVersion.id) return 1
              return Date.parse(right.updatedAt) - Date.parse(left.updatedAt)
            }).map((review) => {
              const isAssignedToActiveUser = review.reviewerId === activeUserId && review.status === 'requested'
              return (
                <article className="activity-item" key={review.id}>
                  <div>
                    <strong>{userDisplayName(users, review.reviewerId)}</strong>
                    <span className={`status-pill ${review.status}`}>{reviewStatusLabel(review.status)}</span>
                  </div>
                  {review.message ? <p>{review.message}</p> : null}
                  {review.summary ? <p>{review.summary}</p> : null}
                  <small>
                    {review.versionNumber ? `v${review.versionNumber} · ` : ''}
                    Requested by {userDisplayName(users, review.requestedBy)} · {new Date(review.updatedAt).toLocaleString()}
                  </small>
                  {isAssignedToActiveUser ? (
                    <div className="review-response">
                      <textarea
                        rows={3}
                        value={reviewSummaryDrafts[review.id] ?? review.summary}
                        placeholder="Summarize your review..."
                        onChange={(event) => onReviewSummaryChange(review.id, event.target.value)}
                      />
                      <div className="review-response-actions">
                        <button className="secondary-action compact-action" type="button" onClick={() => onCompleteReview(review.id, 'changes_requested')} disabled={isSaving}>
                          Request changes
                        </button>
                        <button className="primary-action" type="button" onClick={() => onCompleteReview(review.id, 'approved')} disabled={isSaving}>
                          <CheckCircle2 size={16} /> Approve
                        </button>
                      </div>
                    </div>
                  ) : null}
                </article>
              )
            })}
          </div>
        ) : (
          <div className="analysis-empty">
            <strong>No reviews yet</strong>
            <span>Ask a teammate to review the current design when it is ready.</span>
          </div>
        )}
      </section>

      <section className="review-card">
        <div className="section-title">Comments</div>
        {comments.length ? (
          <div className="activity-list">
            {comments.map((comment) => (
              <article className="activity-item" key={comment.id}>
                <div>
                  <strong>{userDisplayName(users, comment.authorId)}</strong>
                  <span>{new Date(comment.createdAt).toLocaleString()}</span>
                </div>
                <p>{comment.body}</p>
                {comment.componentId ? <small>Component: {comment.componentId}</small> : null}
                {comment.connectorId ? <small>Connector: {comment.connectorId}</small> : null}
              </article>
            ))}
          </div>
        ) : (
          <div className="analysis-empty">
            <strong>No comments yet</strong>
            <span>Switch to comment mode and capture feedback without changing the canvas.</span>
          </div>
        )}
      </section>
    </div>
  )
}

export function RequestReviewModal({
  users,
  activeUserId,
  version,
  isSaving,
  onCancel,
  onRequest,
}: {
  users: BackendUser[]
  activeUserId: string
  version: BackendDesignVersion | null
  isSaving: boolean
  onCancel: () => void
  onRequest: (reviewerIds: string[], message: string) => void
}) {
  const reviewers = users.filter((user) => user.id !== activeUserId)
  const [reviewerIds, setReviewerIds] = useState<Set<string>>(() => new Set())
  const [message, setMessage] = useState('')
  const selectedReviewerIds = [...reviewerIds]

  function toggleReviewer(reviewerId: string, selected: boolean) {
    setReviewerIds((current) => {
      const next = new Set(current)
      if (selected) {
        next.add(reviewerId)
      } else {
        next.delete(reviewerId)
      }
      return next
    })
  }

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="request-review-modal" role="dialog" aria-modal="true" aria-labelledby="request-review-title">
        <div className="modal-heading">
          <p className="eyebrow">Request review</p>
          <h2 id="request-review-title">Choose reviewers</h2>
          <span>
            {version ? `Version v${version.versionNumber} will move to pending review after you send this request.` : 'Create or save a version before requesting review.'}
          </span>
        </div>
        <div className="reviewer-picker">
          {reviewers.map((user) => (
            <label key={user.id}>
              <input
                type="checkbox"
                checked={reviewerIds.has(user.id)}
                onChange={(event) => toggleReviewer(user.id, event.target.checked)}
              />
              <span>
                <strong>{user.displayName}</strong>
                <small>{roleLabel(user.role)} · {user.email}</small>
              </span>
            </label>
          ))}
        </div>
        <TextareaField label="Message" value={message} onChange={setMessage} />
        <div className="modal-actions">
          <button className="secondary-action" type="button" onClick={onCancel} disabled={isSaving}>
            Cancel
          </button>
          <button className="primary-action" type="button" onClick={() => onRequest(selectedReviewerIds, message)} disabled={isSaving || !version || !selectedReviewerIds.length}>
            <UserPlus size={16} /> Request review
          </button>
        </div>
      </section>
    </div>
  )
}

export function NotificationsModal({
  notifications,
  users,
  onMarkRead,
  onClose,
}: {
  notifications: BackendNotification[]
  users: BackendUser[]
  onMarkRead: (notificationId: string) => void
  onClose: () => void
}) {
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="notifications-modal" role="dialog" aria-modal="true" aria-labelledby="notifications-title">
        <div className="analysis-modal-header">
          <div className="modal-heading">
            <p className="eyebrow">Notifications</p>
            <h2 id="notifications-title">Review activity</h2>
            <span>Comments and review updates for the active dummy user.</span>
          </div>
          <button className="secondary-action" type="button" onClick={onClose}>
            Close
          </button>
        </div>
        <div className="activity-list notification-list">
          {notifications.length ? (
            notifications.map((notification) => (
              <article className={`activity-item ${notification.read ? '' : 'unread'}`} key={notification.id}>
                <div>
                  <strong>{notification.title}</strong>
                  <span>{new Date(notification.createdAt).toLocaleString()}</span>
                </div>
                <p>{notification.body}</p>
                <small>For {userDisplayName(users, notification.userId)}</small>
                {!notification.read ? (
                  <button className="text-button" type="button" onClick={() => onMarkRead(notification.id)}>
                    Mark read
                  </button>
                ) : null}
              </article>
            ))
          ) : (
            <div className="analysis-empty">
              <strong>No notifications</strong>
              <span>Review requests and comments will appear here.</span>
            </div>
          )}
        </div>
      </section>
    </div>
  )
}
