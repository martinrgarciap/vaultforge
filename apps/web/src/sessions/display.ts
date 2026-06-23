export function formatSessionTimestamp(value: string): string {
  const timestamp = new Date(value);

  if (Number.isNaN(timestamp.getTime())) {
    return value;
  }

  return timestamp.toLocaleString();
}

function browserName(userAgent: string): string | undefined {
  if (userAgent.includes("Thunder Client")) {
    return "Thunder Client";
  }

  if (userAgent.includes("PostmanRuntime")) {
    return "Postman";
  }

  if (userAgent.includes("Edg/")) {
    return "Microsoft Edge";
  }

  if (userAgent.includes("Chrome/")) {
    return "Chrome";
  }

  if (userAgent.includes("Firefox/")) {
    return "Firefox";
  }

  if (userAgent.includes("Safari/") && userAgent.includes("Version/")) {
    return "Safari";
  }

  if (userAgent.toLowerCase().startsWith("curl/")) {
    return "curl";
  }

  return undefined;
}

function platformName(userAgent: string): string | undefined {
  if (userAgent.includes("iPhone")) {
    return "iPhone";
  }

  if (userAgent.includes("iPad")) {
    return "iPad";
  }

  if (userAgent.includes("Android")) {
    return "Android";
  }

  if (userAgent.includes("Mac OS X") || userAgent.includes("Macintosh")) {
    return "macOS";
  }

  if (userAgent.includes("Windows NT")) {
    return "Windows";
  }

  if (userAgent.includes("Linux")) {
    return "Linux";
  }

  return undefined;
}

export function sessionClientLabel(userAgent: string): string {
  const normalizedUserAgent = userAgent.trim();

  if (normalizedUserAgent === "") {
    return "Unknown client";
  }

  const browser = browserName(normalizedUserAgent);
  const platform = platformName(normalizedUserAgent);

  if (browser && platform) {
    return `${browser} on ${platform}`;
  }

  return browser ?? platform ?? "Unrecognized client";
}

export function sessionUserAgentDescription(userAgent: string): string {
  const normalizedUserAgent = userAgent.trim();

  return normalizedUserAgent === ""
    ? "User agent was not provided."
    : normalizedUserAgent;
}
