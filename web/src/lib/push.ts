import { api } from "../api/client";

export type PushState = "unsupported" | "disabled" | "denied" | "ready" | "subscribed";

function urlBase64ToUint8Array(base64: string): Uint8Array {
  const padding = "=".repeat((4 - (base64.length % 4)) % 4);
  const b64 = (base64 + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(b64);
  return Uint8Array.from([...raw].map((c) => c.charCodeAt(0)));
}

/** Текущее состояние push-подписки для кнопки на дашборде. */
export async function getPushState(): Promise<PushState> {
  if (!("serviceWorker" in navigator) || !("PushManager" in window)) {
    return "unsupported";
  }
  const { enabled } = await api<{ enabled: boolean; key: string }>("/api/push/vapid-key");
  if (!enabled) return "disabled";
  if (Notification.permission === "denied") return "denied";

  const reg = await navigator.serviceWorker.ready;
  const sub = await reg.pushManager.getSubscription();
  return sub ? "subscribed" : "ready";
}

/** Запросить разрешение, подписаться и зарегистрировать подписку на сервере. */
export async function subscribePush(): Promise<PushState> {
  const { enabled, key } = await api<{ enabled: boolean; key: string }>("/api/push/vapid-key");
  if (!enabled) return "disabled";

  const permission = await Notification.requestPermission();
  if (permission !== "granted") return "denied";

  const reg = await navigator.serviceWorker.ready;
  const sub =
    (await reg.pushManager.getSubscription()) ??
    (await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(key),
    }));

  await api("/api/push/subscribe", {
    method: "POST",
    body: JSON.stringify(sub.toJSON()),
  });
  return "subscribed";
}
