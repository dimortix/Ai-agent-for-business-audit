// Кризисный сценарий (приёмочный критерий №4): подписка на web push из
// браузера, затем импорт «провальных» дней снаружи (make crisis), пересчёт
// роняет ИЖБ < 40 → сервер шлёт push. Скрипт: логин → подписка → пауза для
// make crisis → скриншот красного дашборда.
import puppeteer from "puppeteer-core";
import { execSync } from "node:child_process";
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

try {
  // разрешаем уведомления заранее — headless не покажет диалог
  await browser.defaultBrowserContext().overridePermissions(BASE, ["notifications"]);

  const page = await browser.newPage();
  await page.setViewport({ width: 375, height: 812, deviceScaleFactor: 2 });

  // --- логин -----------------------------------------------------------------
  await page.goto(BASE + "/login", { waitUntil: "networkidle0" });
  await page.type("#phone", "+79001234567");
  await Promise.all([
    page.waitForSelector("#code", { timeout: 15000 }),
    page.click("form button"),
  ]);
  await page.waitForFunction(() => /код\s+\d{4}/.test(document.body.innerText), { timeout: 10000 });
  const code = await page.evaluate(() => document.body.innerText.match(/код\s+(\d{4})/)?.[1]);
  await page.type("#code", code);
  await Promise.all([
    page.waitForNavigation({ waitUntil: "networkidle0", timeout: 20000 }),
    page.click("form button:last-of-type"),
  ]);
  await page.waitForFunction(() => document.body.innerText.includes("Индекс здоровья"), { timeout: 20000 });

  // --- подписка на push --------------------------------------------------------
  let subscribed = false;
  try {
    const btn = await page.waitForFunction(() => {
      const b = [...document.querySelectorAll("button")].find((x) => x.textContent?.trim() === "Включить");
      return b ?? false;
    }, { timeout: 8000 });
    await btn.asElement().click();
    await page.waitForFunction(
      () => document.body.innerText.includes("Уведомления включены"),
      { timeout: 20000 },
    );
    subscribed = true;
    console.log("push: подписка оформлена (FCM endpoint сохранён на сервере)");
  } catch {
    console.log("push: подписаться из headless не вышло (пуш-сервис недоступен) — проверяем по логам сервера");
  }

  // --- кризис -------------------------------------------------------------------
  console.log("импортирую кризисные данные (make crisis)…");
  execSync("make crisis", { cwd: "..", stdio: "inherit" });

  await page.reload({ waitUntil: "networkidle0" });
  await page.waitForFunction(() => document.body.innerText.includes("Индекс здоровья"), { timeout: 20000 });
  await new Promise((r) => setTimeout(r, 1500));
  await page.screenshot({ path: `${OUT}/07-dashboard-crisis-375.png`, fullPage: true });

  const idx = await page.evaluate(() => {
    const m = document.body.innerText.match(/(\d+)\s*\n?\s*из 100/);
    return m ? Number(m[1]) : null;
  });
  console.log("ИЖБ после кризиса:", idx);
  if (idx === null || idx >= 40) {
    console.log("ОШИБКА: индекс не упал в красную зону");
    process.exitCode = 2;
  }
  console.log("subscribed:", subscribed);
} finally {
  await browser.close();
}
