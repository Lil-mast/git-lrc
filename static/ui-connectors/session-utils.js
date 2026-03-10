export const UNKNOWN_USER_LABEL = 'Unknown User';

function normalizeIdentityValue(value) {
  return String(value || '').trim().replace(/\s+/g, ' ').toLowerCase();
}

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

export function dedupeIdentityLines(values) {
  const lines = [];
  const seen = new Set();

  for (const value of values || []) {
    const text = String(value || '').trim();
    if (!text) {
      continue;
    }

    const normalized = normalizeIdentityValue(text);
    if (!normalized || seen.has(normalized)) {
      continue;
    }

    seen.add(normalized);
    lines.push(text);
  }

  return lines;
}
