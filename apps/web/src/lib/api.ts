export interface Principal {
  user_id: string;
  organization_id: string;
  display_name: string;
  permissions: string[];
}

export interface RoleRef { id: string; name: string }
export interface UserRecord { id: string; email?: string; display_name: string; status: "active" | "disabled"; roles: RoleRef[]; last_login_at?: string; created_at: string }
export interface RoleRecord { id: string; name: string; description: string; is_system: boolean; permissions: string[]; menu_ids: string[]; created_at: string }
export interface PermissionRecord { code: string; description: string }
export interface MenuRecord { id: string; parent_id?: string; code: string; name: string; path: string; icon: string; sort_order: number; required_permission?: string; status: "active" | "hidden" }
export interface SettingRecord { id: string; namespace: string; key: string; value: unknown; description: string; updated_at: string }
export interface StoreRecord { id: string; name: string; domain: string; status: string; last_sync?: string; primary_domain?: string; currency?: string; timezone?: string; plan_name?: string; scopes?: string[]; expires_at?: string; last_error?: string }
export interface IntegrationConfig { provider: string; public_config: Record<string, unknown>; enabled: boolean; secret_configured: boolean; updated_at?: string }
export interface DingTalkUser { user_id: string; display_name: string; email?: string; status: string; provider_user_id: string; last_login_at?: string }

interface Envelope<T> { data: T }
interface ErrorEnvelope { error?: { code?: string; message?: string } }

export class APIError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
  }
}

export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  if (options.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  const response = await fetch(`/backend${path}`, {
    ...options,
    headers,
    credentials: "include",
    cache: "no-store",
  });
  if (response.status === 204) return undefined as T;
  const payload = (await response.json().catch(() => ({}))) as Envelope<T> & ErrorEnvelope;
  if (!response.ok) {
    throw new APIError(
      response.status,
      payload.error?.code ?? "request_failed",
      payload.error?.message ?? "请求失败",
    );
  }
  return payload.data;
}

export const can = (principal: Principal | null, permission: string) =>
  Boolean(principal?.permissions.includes("*") || principal?.permissions.includes(permission));
