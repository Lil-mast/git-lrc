import { LOGO_DATA_URI } from '/static/components/utils.js';
import { getDisplayName, getInitials } from '/static/ui-connectors/session-utils.js';

const { html } = window.preact;

export function HeaderNav({ activePath, session, reauthInProgress, onReauthenticate }) {
  const homeActive = activePath === '/home';
  const connectorsActive = activePath.startsWith('/connectors');
  const authenticated = Boolean(session && session.authenticated);
  const displayName = getDisplayName(session);
  const sessionHint = session && session.user_email ? session.user_email : (session && session.message ? session.message : 'Sign in required');
  const orgLabel = (session && session.org_name) || (session && session.org_id ? `Org #${session.org_id}` : 'No organization');
  const avatarURL = session && session.avatar_url ? session.avatar_url : '';
  const initials = getInitials(session);

  return html`
    <div>
      <div class="header">
        <div class="brand">
          <div class="logo-wrap">
            <img alt="git-lrc" src=${LOGO_DATA_URI} />
          </div>
          <div class="brand-text">
            <h1>git-lrc</h1>
            <div class="meta">Manage your git-lrc</div>
          </div>
        </div>

        <div class="header-right">
          ${authenticated ? html`
            <a class="profile-chip" href="#/profile" title=${sessionHint}>
              ${avatarURL
                ? html`<img class="profile-chip-avatar" src=${avatarURL} alt=${displayName} />`
                : html`<div class="profile-chip-avatar profile-chip-fallback">${initials}</div>`}
              <div class="profile-chip-text">
                <div class="profile-chip-name">${displayName}</div>
                <div class="profile-chip-meta">${sessionHint}</div>
                <div class="profile-chip-org">${orgLabel}</div>
              </div>
            </a>
          ` : html`
            <div class="header-auth-actions">
              <div class="session-pill session-bad" title=${sessionHint}>Not Authenticated</div>
              <button class="secondary" disabled=${reauthInProgress} onClick=${onReauthenticate}>
                ${reauthInProgress ? 'Signing in...' : 'Sign in'}
              </button>
            </div>
          `}
        </div>
      </div>

      <nav class="ui-nav" aria-label="git-lrc manager navigation">
        <span class="nav-label">Menu</span>
        <a href="#/home" class=${`nav-link ${homeActive ? 'active' : ''}`}>Home</a>
        <a href="#/connectors" class=${`nav-link ${connectorsActive ? 'active' : ''}`}>AI Connectors</a>
      </nav>
    </div>
  `;
}
