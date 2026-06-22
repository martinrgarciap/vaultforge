const csrfCookieName = "vaultforge_csrf";
const csrfTokenPattern = /^vf_csrf_[A-Za-z0-9_-]{43}$/;

export function readCSRFToken(): string | null {
  const cookies = document.cookie.split(";");

  for (const cookie of cookies) {
    const trimmedCookie = cookie.trim();
    const separatorIndex = trimmedCookie.indexOf("=");

    if (separatorIndex < 0) {
      continue;
    }

    const name = trimmedCookie.slice(0, separatorIndex);

    if (name !== csrfCookieName) {
      continue;
    }

    const encodedValue = trimmedCookie.slice(separatorIndex + 1);

    try {
      const value = decodeURIComponent(encodedValue);

      return csrfTokenPattern.test(value) ? value : null;
    } catch {
      return null;
    }
  }

  return null;
}
