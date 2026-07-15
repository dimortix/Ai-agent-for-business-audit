/// <reference lib="webworker" />
// Сервис-воркер Альфа-Пульс: precache оболочки (PWA), офлайн-fallback,
// приём push-уведомлений.
import { clientsClaim } from "workbox-core";
import { createHandlerBoundToURL, precacheAndRoute } from "workbox-precaching";
import { NavigationRoute, registerRoute } from "workbox-routing";
import { NetworkFirst } from "workbox-strategies";

declare let self: ServiceWorkerGlobalScope;

self.skipWaiting();
clientsClaim();

// Статика собранного приложения (заполняется vite-plugin-pwa на сборке).
precacheAndRoute(self.__WB_MANIFEST);

// SPA-навигация → index.html (кроме API).
registerRoute(
  new NavigationRoute(createHandlerBoundToURL("index.html"), {
    denylist: [/^\/api\//],
  }),
);

// Данные дашборда/аналитики: сеть с фолбэком на кэш (для офлайна).
registerRoute(
  ({ url, request }) =>
    request.method === "GET" &&
    (url.pathname === "/api/dashboard" ||
      url.pathname.startsWith("/api/analytics") ||
      url.pathname.startsWith("/api/advice")),
  new NetworkFirst({ cacheName: "ap-api", networkTimeoutSeconds: 4 }),
);

interface PushPayload {
  title?: string;
  body?: string;
  url?: string;
}

self.addEventListener("push", (event) => {
  let data: PushPayload = {};
  try {
    data = (event.data?.json() ?? {}) as PushPayload;
  } catch {
    data = { body: event.data?.text() };
  }
  event.waitUntil(
    self.registration.showNotification(data.title ?? "Альфа-Пульс", {
      body: data.body ?? "",
      icon: "/icons/icon-192.png",
      badge: "/icons/icon-192.png",
      data: { url: data.url ?? "/" },
    }),
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const url = (event.notification.data as { url?: string })?.url ?? "/";
  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((clients) => {
      for (const client of clients) {
        if ("focus" in client) {
          void client.navigate(url);
          return client.focus();
        }
      }
      return self.clients.openWindow(url);
    }),
  );
});
