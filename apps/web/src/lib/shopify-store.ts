const shopifyDomainPattern = /^[a-z0-9][a-z0-9-]*\.myshopify\.com$/i;

export function normalizeShopifyStoreDomain(value: string): string | null {
  const trimmed = value.trim();
  if (!trimmed) return null;

  let domain = trimmed;
  if (/^https?:\/\//i.test(trimmed)) {
    try {
      domain = new URL(trimmed).hostname;
    } catch {
      return null;
    }
  }

  domain = domain.toLowerCase();
  return shopifyDomainPattern.test(domain) ? domain : null;
}

const statusLabels: Record<string, string> = {
  pending: "等待授权",
  connected: "已连接",
  action_required: "需重新授权",
  disconnected: "已断开",
};

export function shopifyStoreStatusLabel(status: string): string {
  return statusLabels[status] ?? status;
}
