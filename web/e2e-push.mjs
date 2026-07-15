// Живая демонстрация web push: открывает ВИДИМОЕ окно Chrome, логинится,
// подписывается на уведомления, сбрасывает антиспам и дёргает пересчёт —
// сервер шлёт push через FCM, Chrome показывает системное уведомление.
// Окно остаётся открытым (browser.disconnect в конце).
import puppeteer from "puppeteer-core";
import { execSync } from "node:child_process";
import { mkdirSync } from "node:fs";

const BASE = process.env.BASE_URL ?? "http://localhost:8080";
const ADMIN = process.env.ADMIN_TOKEN ?? "alfa-admin";
const OUT = process.argv[2] ?? "e2e-shots";
const CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";

mkdirSync(OUT, { recursive: true });

// Подключение к уже запущенному Chrome через DevTools-протокол (CDP).
// Инстанс поднимается снаружи: см. README/командную строку с
// --remote-debugging-port=9223 и отдельным --user-data-dir.
const DEVTOOLS = process.env.DEVTOOLS_URL ?? "http://127.0.0.1:9223";
const browser = await puppeteer.connect({
  browserURL: DEVTOOLS,
  defaultViewport: null,
});
void CHROME; // путь нужен только внешней команде запуска

try {
  await browser.defaultBrowserContext().overridePermissions(BASE, ["notifications"]);
  const page = await browser.newPage();

  // --- логин -----------------------------------------------------------------
  await page.goto(BASE + "/login", { waitUntil: "networkidle0" });
  await page.type("#phone", "+79001234567", { delay: 40 });
  await Promise.all([
    page.waitForSelector("#code", { timeout: 15000 }),
    page.click("form button"),
  ]);
  await page.waitForFunction(() => /код\s+\d{4}/.test(document.body.innerText), { timeout: 10000 });
  const code = await page.evaluate(() => document.body.innerText.match(/код\s+(\d{4})/)?.[1]);
  await page.type("#code", code, { delay: 80 });
  await Promise.all([
    page.waitForNavigation({ waitUntil: "networkidle0", timeout: 20000 }),
    page.click("form button:last-of-type"),
  ]);
  await page.waitForFunction(() => document.body.innerText.includes("Индекс здоровья"), { timeout: 20000 });
  console.log("вход выполнен, дашборд открыт");

  // --- подписка на push --------------------------------------------------------
  const already = await page.evaluate(() => document.body.innerText.includes("Уведомления включены"));
  if (!already) {
    const btn = await page.waitForFunction(() => {
      const b = [...document.querySelectorAll("button")].find((x) => x.textContent?.trim() === "Включить");
      return b ?? false;
    }, { timeout: 10000 });
    await btn.asElement().click();
    await page.waitForFunction(() => document.body.innerText.includes("Уведомления включены"), { timeout: 25000 });
  }
  const endpoint = await page.evaluate(async () => {
    const reg = await navigator.serviceWorker.ready;
    const sub = await reg.pushManager.getSubscription();
    return sub?.endpoint ?? null;
  });
  if (!endpoint) throw new Error("подписка не оформилась");
  console.log("push-подписка активна:", endpoint.slice(0, 60) + "…");

  // --- триггер тревоги ----------------------------------------------------------
  execSync("make reset-alarm", { cwd: "..", stdio: "ignore" });
  const list = await (await fetch(BASE + "/api/admin/participants", {
    headers: { "X-Admin-Token": ADMIN },
  })).json();
  const pid = list.items.find((i) => i.group_type === "B").id;
  const rec = await (await fetch(`${BASE}/api/admin/recalculate/${pid}`, {
    method: "POST",
    headers: { "X-Admin-Token": ADMIN },
  })).json();
  console.log("пересчёт: индекс =", rec.health_index, "→ сервер шлёт push");

  // --- ждём доставку через FCM ---------------------------------------------------
  let shown = null;
  for (let i = 0; i < 30 && !shown; i++) {
    await new Promise((r) => setTimeout(r, 1000));
    shown = await page.evaluate(async () => {
      const reg = await navigator.serviceWorker.ready;
      const ns = await reg.getNotifications();
      return ns.length ? { title: ns[0].title, body: ns[0].body } : null;
    });
  }
  if (shown) {
    console.log("УВЕДОМЛЕНИЕ ПОКАЗАНО:");
    console.log("  заголовок:", shown.title);
    console.log("  текст:", shown.body);
  } else {
    console.log("уведомление не зафиксировано за 30с — проверьте логи сервера");
    process.exitCode = 2;
  }
  await page.screenshot({ path: `${OUT}/08-push-live.png` });
} catch (err) {
  // диагностика: что было на экране в момент сбоя
  try {
    const p = (await browser.pages()).at(-1);
    if (p) {
      console.log("страница в момент ошибки:", p.url());
      console.log((await p.evaluate(() => document.body.innerText)).slice(0, 400));
      await p.screenshot({ path: `${OUT}/99-push-fail.png` });
    }
  } catch { /* ignore */ }
  throw err;
} finally {
  await browser.disconnect(); // окно остаётся открытым для пользователя
}
