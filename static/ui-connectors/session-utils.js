export const UNKNOWN_USER_LABEL = 'Unknown User';

export function getDisplayName(session) {
  if (!session) {
    return UNKNOWN_USER_LABEL;
  }

  const displayName = (session.display_name || '').trim();
  if (displayName) {
    return displayName;
  }

  const first = (session.first_name || '').trim();
  const last = (session.last_name || '').trim();
  const combined = `${first} ${last}`.trim();
  if (combined) {
    return combined;
  }

  const email = (session.user_email || '').trim();
  if (email) {
    return email;
  }

  return UNKNOWN_USER_LABEL;
}

export function getInitials(session) {
  const displayName = getDisplayName(session);
  if (!displayName || displayName === UNKNOWN_USER_LABEL) {
    return 'U';
  }

  return displayName
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part.charAt(0).toUpperCase())
    .join('') || 'U';
}
