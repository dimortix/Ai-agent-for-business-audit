// E2E-прогон Альфа.Пульс в headless Chrome (puppeteer-core):
// логин по OTP (debug_code из dev-режима) → дашборд → аналитика → советы,
// мобильный вьюпорт 375px (приёмочный критерий №5), скриншоты в out-каталог.
//
// Запуск: node scripts/e2e.mjs [outDir]   (из корня, нужен web/node_modules)
import puppeteer from "puppeteer-core";
import { mkdirSync } from "node:fs";

const BASE = process.env.BASE_URL ?? "http://localhost:8080";
const OUT = process.argv[2] ?? "e2e-shots";
const CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";

mkdirSync(OUT, { recursive: true });

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: "shell",
  args: ["--no-first-run", "--disable-extensions"],
});

const errors = [];
try {
  const page = await browser.newPage();
  await page.setViewport({ width: 375, height: 812, deviceScaleFactor: 2 });
  page.on("pageerror", (e) => errors.push("pageerror: " + e.message));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push("console: " + m.text());
  });

  // --- логин ---------------------------------------------------------------
  await page.goto(BASE + "/login", { waitUntil: "networkidle0" });
  await page.screenshot({ path: `${OUT}/01-login-375.png` });

  await page.type("#phone", "+79001234567");
  await Promise.all([
    page.waitForSelector("#code", { timeout: 15000 }),
    page.click("button[type=submit], form button"),
  ]);

  // dev-режим показывает код на экране: «демо-режим: код XXXX»
  await page.waitForFunction(() => /код\s+\d{4}/.test(document.body.innerText), { timeout: 10000 });
  const code = (await page.evaluate(() => document.body.innerText.match(/код\s+(\d{4})/)?.[1])) ?? "";
  if (!/^\d{4}$/.test(code)) throw new Error("не удалось прочитать debug-код с экрана");
  console.log("OTP со страницы:", code);
  await page.screenshot({ path: `${OUT}/02-code-375.png` });

  await page.type("#code", code);
  await Promise.all([
    page.waitForNavigation({ waitUntil: "networkidle0", timeout: 20000 }),
    page.click("form button[type=submit], form button:last-of-type"),
  ]);

  // --- дашборд ---------------------------------------------------------------
  await page.waitForFunction(
    () => document.body.innerText.includes("Индекс здоровья бизнеса"),
    { timeout: 20000 },
  );
  await new Promise((r) => setTimeout(r, 1200)); // дорисовка графика и гейджа
  await page.screenshot({ path: `${OUT}/03-dashboard-375.png`, fullPage: true });

  const text = await page.evaluate(() => document.body.innerText);
  for (const marker of ["Индекс здоровья бизнеса", "факт", "прогноз", "кассовый разрыв"]) {
    if (!text.toLowerCase().includes(marker.toLowerCase())) {
      throw new Error(`на дашборде не найден блок: «${marker}»`);
    }
  }
  console.log("дашборд: гейдж, график и карточка разрыва на месте");

  // --- аналитика -------------------------------------------------------------
  await page.goto(BASE + "/analytics", { waitUntil: "networkidle0" });
  await page.waitForFunction(() => document.body.innerText.includes("Чистая выручка"), { timeout: 15000 });
  await new Promise((r) => setTimeout(r, 800));
  await page.screenshot({ path: `${OUT}/04-analytics-375.png`, fullPage: true });

  // --- советы ---------------------------------------------------------------
  await page.goto(BASE + "/advice", { waitUntil: "networkidle0" });
  await page.waitForFunction(() => document.body.innerText.includes("Актуальные"), { timeout: 15000 });
  await new Promise((r) => setTimeout(r, 500));
  await page.screenshot({ path: `${OUT}/05-advice-375.png`, fullPage: true });

  // --- десктоп-вид дашборда ----------------------------------------------------
  await page.setViewport({ width: 1280, height: 800, deviceScaleFactor: 1 });
  await page.goto(BASE + "/", { waitUntil: "networkidle0" });
  await page.waitForFunction(() => document.body.innerText.includes("Индекс здоровья"), { timeout: 15000 });
  await new Promise((r) => setTimeout(r, 1000));
  await page.screenshot({ path: `${OUT}/06-dashboard-desktop.png`, fullPage: true });

  // --- сервис-воркер (PWA) -----------------------------------------------------
  const swState = await page.evaluate(async () => {
    const reg = await navigator.serviceWorker?.getRegistration();
    return reg ? (reg.active ? "active" : "installing") : "none";
  });
  console.log("service worker:", swState);

  const relevant = errors.filter((e) => !e.includes("favicon"));
  if (relevant.length) {
    console.log("console-ошибки:", relevant.join("\n"));
    process.exitCode = 2;
  } else {
    console.log("console: без ошибок");
  }
  console.log("скриншоты в", OUT + "/");
} finally {
  await browser.close();
}
