import type { Participant } from "./types";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

let refreshing: Promise<boolean> | null = null;

async function tryRefresh(): Promise<boolean> {
  refreshing ??= fetch("/api/auth/refresh", { method: "POST" })
    .then((r) => r.ok)
    .catch(() => false)
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

/** fetch-обёртка: JSON, единый разбор ошибок, авто-refresh при 401. */
export async function api<T>(
  path: string,
  init?: RequestInit,
  retried = false,
): Promise<T> {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    ...init,
  });

  if (res.status === 401 && !path.startsWith("/api/auth/") && !retried) {
    if (await tryRefresh()) return api<T>(path, init, true);
    clearUser();
    throw new ApiError(401, "Сессия истекла");
  }

  if (!res.ok) {
    let msg = `Ошибка запроса (${res.status})`;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) msg = body.error;
    } catch {
      /* тело не JSON — оставляем дефолт */
    }
    throw new ApiError(res.status, msg);
  }

  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

/* --- локальное состояние «кто вошёл» (для маршрутизации и шапки) --------- */

const USER_KEY = "ap_user";

export function saveUser(p: Participant) {
  localStorage.setItem(USER_KEY, JSON.stringify(p));
}

export function loadUser(): Participant | null {
  try {
    const raw = localStorage.getItem(USER_KEY);
    return raw ? (JSON.parse(raw) as Participant) : null;
  } catch {
    return null;
  }
}

export function clearUser() {
  localStorage.removeItem(USER_KEY);
}

export async function logout() {
  try {
    await api("/api/auth/logout", { method: "POST" });
  } finally {
    clearUser();
  }
}
