// Полный E2E-прогон «Альфа-Пульс» (headless Chrome, puppeteer-core):
// логин B → дашборд → советы (Сделано) → аналитика (пресеты+таблица) →
// выход → логин A (контрольная группа) → админка.
// Запуск: cd web && node e2e-full.mjs
import puppeteer from "puppeteer-core";
import { mkdirSync } from "node:fs";

const BASE = process.env.BASE_URL ?? "http://localhost:8080";
const OUT =
  "/private/tmp/claude-501/-Users-dimortix--------------------/ba639b69-1c4e-4126-ba83-3ca90690a049/scratchpad/shots-agent";
const CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";

mkdirSync(OUT, { recursive: true });

const results = [];
const consoleErrors = [];
let currentStep = "init";

function record(step, ok, note) {
  results.push({ step, ok, note });
  console.log(`${ok ? "PASS" : "FAIL"} | ${step} | ${note}`);
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: "shell",
  args: ["--no-first-run", "--disable-extensions"],
});

const page = await browser.newPage();
await page.setViewport({ width: 375, height: 812, deviceScaleFactor: 2 });
page.on("pageerror", (e) =>
  consoleErrors.push(`[${currentStep}] pageerror: ${e.message}`),
);
page.on("console", (m) => {
  if (m.type() === "error" && !m.text().includes("favicon"))
    consoleErrors.push(`[${currentStep}] console: ${m.text()}`);
});

async function shot(name, fullPage = true) {
  const path = `${OUT}/${name}`;
  await page.screenshot({ path, fullPage });
  console.log("  shot:", path);
}

const bodyText = () => page.evaluate(() => document.body.innerText);

async function waitText(needle, timeout = 15000) {
  await page.waitForFunction(
    (t) => document.body.innerText.toLowerCase().includes(t.toLowerCase()),
    { timeout },
    needle,
  );
}

async function clickByText(selector, needle) {
  const ok = await page.evaluate(
    (sel, t) => {
      const el = [...document.querySelectorAll(sel)].find((e) =>
        (e.innerText || "").trim().toLowerCase().includes(t.toLowerCase()),
      );
      if (!el) return false;
      el.click();
      return true;
    },
    selector,
    needle,
  );
  if (!ok) throw new Error(`не найден ${selector} с текстом «${needle}»`);
}

async function login(phone) {
  await page.goto(BASE + "/login", { waitUntil: "networkidle0" });
  await page.waitForSelector("#phone", { timeout: 15000 });
  await page.type("#phone", phone);
  await clickByText("form button", "Получить код");
  await page.waitForSelector("#code", { timeout: 15000 });
  await page.waitForFunction(
    () => /код\s+\d{4}/.test(document.body.innerText),
    { timeout: 10000 },
  );
  const code = await page.evaluate(
    () => document.body.innerText.match(/код\s+(\d{4})/)?.[1],
  );
  if (!/^\d{4}$/.test(code ?? ""))
    throw new Error("не удалось прочитать OTP с экрана");
  console.log(`  OTP для ${phone}: ${code}`);
  await page.type("#code", code);
  await clickByText("form button", "Войти");
}

async function step(name, fn) {
  currentStep = name;
  try {
    await fn();
  } catch (e) {
    record(name, false, e.message);
    try {
      await shot(`fail-${name.replace(/[^a-z0-9а-яё]+/gi, "-")}.png`, true);
    } catch {}
  }
}

// ---------- 1. Вход участника B --------------------------------------------
await step("1-login-B", async () => {
  await login("+79001234567");
  await waitText("Индекс здоровья бизнеса", 20000);
  const url = page.url();
  const redirected = url === BASE + "/" || url === BASE + "";
  record(
    "1-login-B",
    redirected,
    redirected
      ? "OTP прочитан с экрана, вход выполнен, редирект на дашборд (" + url + ")"
      : "дашборд отрисован, но URL неожиданный: " + url,
  );
});

// ---------- 2. Дашборд B -----------------------------------------------------
await step("2-dashboard-B", async () => {
  await waitText("Индекс здоровья бизнеса", 20000);
  await sleep(1500); // дорисовка гейджа/графика
  const d = await page.evaluate(() => {
    const t = document.body.innerText;
    const svg = document.querySelector('svg[aria-label*="Индекс здоровья"]');
    const big = [...document.querySelectorAll("div")].find((e) =>
      e.className.includes?.("text-5xl"),
    );
    const gap = [...document.querySelectorAll("a.card")].find((e) =>
      e.innerText.includes("Возможен кассовый разрыв"),
    );
    return {
      aria: svg?.getAttribute("aria-label") ?? null,
      bigValue: big?.innerText.trim() ?? null,
      hasTitle: t.includes("Индекс здоровья бизнеса"),
      hasUrgent: t.includes("Нужны срочные меры"),
      hasGapText: t.includes("Возможен кассовый разрыв"),
      gapIsRed: !!gap && (gap.getAttribute("style") ?? "").includes("crit"),
      hasChart: t.includes("Выручка: факт и прогноз 14 дней"),
      hasLegend:
        t.includes("факт") && t.includes("прогноз") && t.includes("интервал 80%"),
      hasModel: t.includes("модель: Prophet (ML)"),
      hasTodo: t.includes("Что сделать сейчас"),
    };
  });
  await shot("02-dashboard-b-full.png", true);

  const checks = [
    ["гейдж «Индекс здоровья бизнеса»", d.hasTitle],
    ["значение индекса = 6", d.bigValue === "6" && (d.aria ?? "").includes("здоровья 6 из 100")],
    ["ярлык «Нужны срочные меры»", d.hasUrgent],
    ["красная карточка «Возможен кассовый разрыв»", d.hasGapText && d.gapIsRed],
    ["график «Выручка: факт и прогноз 14 дней»", d.hasChart],
    ["легенда факт/прогноз/интервал 80%", d.hasLegend],
    ["подпись «модель: Prophet (ML)»", d.hasModel],
    ["секция «Что сделать сейчас»", d.hasTodo],
  ];
  const failed = checks.filter(([, ok]) => !ok).map(([n]) => n);
  record(
    "2-dashboard-B",
    failed.length === 0,
    failed.length === 0
      ? `все 8 проверок ок (гейдж aria: «${d.aria}»)`
      : "нет: " + failed.join("; ") + ` [aria=${d.aria}, big=${d.bigValue}]`,
  );
});

// ---------- 3. Советы --------------------------------------------------------
await step("3-advice", async () => {
  await page.goto(BASE + "/advice", { waitUntil: "networkidle0" });
  await waitText("Актуальные");
  await page.waitForFunction(
    () => !document.querySelector('[role="status"]'),
    { timeout: 15000 },
  );
  await sleep(600);

  const readCards = () =>
    page.evaluate(() =>
      [...document.querySelectorAll("main .card")]
        .map((c) => ({
          title:
            c.querySelector("span.text-xs")?.innerText.trim().toLowerCase() ?? "",
          msg: c.querySelector("p")?.innerText.trim() ?? "",
          hasDone: [...c.querySelectorAll("button")].some((b) =>
            b.innerText.includes("Сделано"),
          ),
          isDone: c.innerText.includes("выполнено"),
        }))
        .filter((c) => c.title || c.msg),
    );

  const before = await readCards();
  await shot("03-advice-active-before.png", true);
  const titles = before.map((c) => c.title);
  const expected = ["кассовый разрыв", "выручка падает", "средний чек", "меньше клиентов"];
  const matched = expected.filter((e) => titles.some((t) => t.includes(e)));

  // жмём «Сделано» ровно на одной карточке НЕ про кассовый разрыв
  const target = before.find(
    (c) => c.hasDone && !c.title.includes("кассовый разрыв"),
  );
  if (!target) throw new Error("нет карточки с кнопкой «Сделано» кроме кассового разрыва");
  const key = target.msg.slice(0, 40);
  await page.evaluate((k) => {
    const card = [...document.querySelectorAll("main .card")].find((c) =>
      (c.querySelector("p")?.innerText ?? "").startsWith(k),
    );
    const btn = [...card.querySelectorAll("button")].find((b) =>
      b.innerText.includes("Сделано"),
    );
    btn.click();
  }, key);

  // ждём, пока карточка уйдёт из «Актуальные»
  await page.waitForFunction(
    (k) => !document.body.innerText.includes(k),
    { timeout: 15000 },
    key,
  );
  await sleep(600);
  const after = await readCards();
  await shot("04-advice-active-after.png", true);
  const goneFromActive = !after.some((c) => c.msg.startsWith(key));

  // фильтр «Выполненные»
  await clickByText("button.chip", "Выполненные");
  await page.waitForFunction(
    (k) => document.body.innerText.includes(k),
    { timeout: 15000 },
    key,
  );
  await sleep(600);
  const done = await readCards();
  await shot("05-advice-done.png", true);
  const doneCard = done.find((c) => c.msg.startsWith(key));

  const ok =
    before.length >= 3 &&
    goneFromActive &&
    after.length === before.length - 1 &&
    !!doneCard &&
    doneCard.isDone;
  record(
    "3-advice",
    ok,
    `актуальных до: ${before.length} (типы: ${matched.join(", ")}); ` +
      `«Сделано» на «${target.title}» → из актуальных ушла: ${goneFromActive}, ` +
      `осталось ${after.length}; в «Выполненные» с пометкой: ${!!doneCard && doneCard.isDone}`,
  );
});

// ---------- 4. Аналитика -----------------------------------------------------
await step("4-analytics", async () => {
  await page.goto(BASE + "/analytics", { waitUntil: "networkidle0" });
  await waitText("Чистая выручка");
  await sleep(800);

  const readKpi = () =>
    page.evaluate(() => {
      const cards = [...document.querySelectorAll("main .card")].slice(0, 4);
      return cards
        .map((c) => {
          const label = c.querySelector(".text-xs")?.innerText.trim();
          const value = c.querySelector(".text-xl")?.innerText.trim();
          return label && value ? `${label}=${value}` : null;
        })
        .filter(Boolean)
        .join("; ");
    });

  const kpi = {};
  for (const preset of ["7 дней", "30 дней", "90 дней"]) {
    await clickByText("button.chip", preset);
    await sleep(400);
    await waitText("Чистая выручка");
    await sleep(700);
    kpi[preset] = await readKpi();
    await shot(`0${preset.startsWith("7") ? 6 : preset.startsWith("30") ? 7 : 8}-analytics-${preset.split(" ")[0]}d.png`, true);
  }
  const distinct = new Set(Object.values(kpi)).size === 3;

  // вид «таблица»
  await clickByText("button.chip", "таблица");
  await page.waitForSelector("main table", { timeout: 10000 });
  await sleep(500);
  const table = await page.evaluate(() => {
    const ths = [...document.querySelectorAll("main table thead th")].map((e) =>
      e.innerText.trim(),
    );
    const rows = document.querySelectorAll("main table tbody tr").length;
    const firstRow = [...(document.querySelector("main table tbody tr")?.cells ?? [])]
      .map((c) => c.innerText.trim())
      .join(" | ");
    return { ths, rows, firstRow };
  });
  await shot("09-analytics-table.png", true);

  const wantCols = ["Дата", "Выручка", "Возвраты", "Покупки", "Ср. чек"];
  const colsOk = wantCols.every((c) => table.ths.includes(c));
  const ok = distinct && colsOk && table.rows > 0;
  record(
    "4-analytics",
    ok,
    `KPI: 7д[${kpi["7 дней"]}] | 30д[${kpi["30 дней"]}] | 90д[${kpi["90 дней"]}]; ` +
      `различаются: ${distinct}; таблица: колонки ${colsOk ? "ок" : table.ths.join(",")}, строк ${table.rows}, первая: ${table.firstRow}`,
  );
});

// ---------- 5. Выход → вход A ------------------------------------------------
await step("5-logout-login-A", async () => {
  await page.click('header button[title="Выйти"]');
  await page.waitForSelector("#phone", { timeout: 15000 });
  await login("+79007654321");
  await waitText("контрольной группе", 20000);
  await sleep(1200);
  const a = await page.evaluate(() => {
    const t = document.body.innerText;
    return {
      hasControl: t.includes("контрольной группе"),
      hasGauge:
        t.includes("Индекс здоровья") ||
        !!document.querySelector('svg[aria-label*="Индекс здоровья"]'),
      hasForecast: t.includes("факт и прогноз 14 дней") || t.includes("интервал 80%"),
      hasWeekChart: t.includes("Выручка за неделю"),
      url: location.pathname,
    };
  });
  await shot("10-dashboard-a-control.png", true);
  const ok = a.hasControl && !a.hasGauge && !a.hasForecast && a.url === "/";
  record(
    "5-logout-login-A",
    ok,
    `пометка контрольной группы: ${a.hasControl}; гейджа нет: ${!a.hasGauge}; ` +
      `прогноза нет: ${!a.hasForecast}; график недели: ${a.hasWeekChart}; url: ${a.url}`,
  );
});

// ---------- 6. Админка -------------------------------------------------------
await step("6-admin", async () => {
  await page.goto(BASE + "/login", { waitUntil: "networkidle0" });
  await page.waitForSelector('a[href="/admin"]', { timeout: 10000 });
  await page.click('a[href="/admin"]');
  await page.waitForSelector('input[placeholder="X-Admin-Token"]', {
    timeout: 10000,
  });
  await page.type('input[placeholder="X-Admin-Token"]', "alfa-admin");
  await clickByText("button", "Подключиться");
  await page.waitForSelector("table tbody tr", { timeout: 15000 });
  await sleep(600);
  const adm = await page.evaluate(() => {
    const rows = [...document.querySelectorAll("table tbody tr")].map((r) =>
      r.innerText.replace(/\s+/g, " ").trim(),
    );
    return rows;
  });
  await shot("11-admin.png", true);
  const rowB = adm.find((r) => r.includes("Демо Кофе"));
  const rowA = adm.find((r) => r.includes("Контроль"));
  const bOk = !!rowB && rowB.includes("B") && /\d+/.test(rowB.split("B").pop() ?? "");
  const aOk = !!rowA && rowA.includes("A");
  const ok = adm.length === 2 && bOk && aOk;
  record(
    "6-admin",
    ok,
    `строк: ${adm.length}; B: «${rowB ?? "нет"}»; A: «${rowA ?? "нет"}»`,
  );
});

// ---------- 7. Итог по консоли -----------------------------------------------
currentStep = "7-console";
record(
  "7-console",
  consoleErrors.length === 0,
  consoleErrors.length === 0
    ? "console/pageerror ошибок нет (favicon игнорирован)"
    : `${consoleErrors.length} ошибок: ` + consoleErrors.join(" || "),
);

await browser.close();

console.log("\n===== ИТОГ =====");
for (const r of results) console.log(`${r.ok ? "PASS" : "FAIL"} | ${r.step} | ${r.note}`);
process.exitCode = results.some((r) => !r.ok) ? 2 : 0;
