import http from "k6/http";
import ws from "k6/ws";
import { check, sleep, group } from "k6";
import { Counter, Rate, Trend } from "k6/metrics";
import { randomString, randomIntBetween } from "https://jslib.k6.io/k6-utils/1.4.0/index.js";

const BASE_URL = __ENV.API_URL || "http://localhost:8080";

export const options = {
  stages: [
    { duration: "15s", target: 5  },
    { duration: "30s", target: 20 },
    { duration: "1m",  target: 20 },
    { duration: "15s", target: 50 },
    { duration: "15s", target: 0  },
  ],
  thresholds: {
    http_req_duration: ["p(95)<500", "p(99)<1000"],
    http_req_failed: ["rate<0.01"],
    transaction_create_duration: ["p(95)<300"],
    auth_duration:               ["p(95)<200"],
  },
};

const transactionCreateDuration = new Trend("transaction_create_duration", true);
const authDuration               = new Trend("auth_duration",               true);
const wsMessagesReceived         = new Counter("ws_messages_received");
const errorRate                  = new Rate("custom_error_rate");

export function setup() {
  const email    = `perf_setup_${randomString(8)}@test.com`;
  const password = "password123";

  const reg = http.post(
    `${BASE_URL}/auth/register`,
    JSON.stringify({ email, password, name: "Perf User" }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(reg, { "setup: register 201": (r) => r.status === 201 });

  const login = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email, password }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(login, { "setup: login 200": (r) => r.status === 200 });

  const token = login.json("data.token");

  const catHeaders = {
    "Content-Type":  "application/json",
    "Authorization": `Bearer ${token}`,
  };
  const catTypes = [
    { name: "Wynagrodzenie", type: "income",  color: "#22C55E" },
    { name: "Jedzenie",      type: "expense", color: "#EF4444" },
    { name: "Transport",     type: "expense", color: "#F97316" },
  ];
  const categoryIDs = [];
  for (const cat of catTypes) {
    const r = http.post(`${BASE_URL}/categories`, JSON.stringify(cat), { headers: catHeaders });
    if (r.status === 201) {
      categoryIDs.push(r.json("data.id"));
    }
  }

  return { token, email, password, categoryIDs };
}

export default function (data) {
  const vuToken = getOrCreateToken(data);
  if (!vuToken) return;

  const headers = {
    "Content-Type":  "application/json",
    "Authorization": `Bearer ${vuToken}`,
  };

  group("Tworzenie transakcji", () => {
    const types   = ["income", "expense"];
    const amounts = [50, 100, 250, 500, 1000, 2500];

    const body = {
      amount:      amounts[randomIntBetween(0, amounts.length - 1)],
      type:        types[randomIntBetween(0, 1)],
      currency:    "PLN",
      description: `Test transaction ${randomString(6)}`,
      date:        "2026-05-28",
    };

    if (data.categoryIDs && data.categoryIDs.length > 0) {
      body.category_id = data.categoryIDs[randomIntBetween(0, data.categoryIDs.length - 1)];
    }

    const start = Date.now();
    const res = http.post(`${BASE_URL}/transactions`, JSON.stringify(body), { headers });
    transactionCreateDuration.add(Date.now() - start);

    const ok = check(res, {
      "create tx: status 201": (r) => r.status === 201,
      "create tx: has id":     (r) => r.json("data.id") > 0,
      "create tx: amount ok":  (r) => r.json("data.amount") === body.amount,
    });
    errorRate.add(!ok);
  });

  sleep(0.5);

  group("Lista transakcji", () => {
    const page    = randomIntBetween(1, 3);
    const perPage = randomIntBetween(10, 20);

    const res = http.get(
      `${BASE_URL}/transactions?page=${page}&per_page=${perPage}`,
      { headers }
    );
    check(res, {
      "list tx: status 200":   (r) => r.status === 200,
      "list tx: has data":     (r) => Array.isArray(r.json("data")),
      "list tx: has meta":     (r) => r.json("meta") !== null,
      "list tx: resp <200ms":  (r) => r.timings.duration < 200,
    });
  });

  sleep(0.3);

  group("Saldo", () => {
    const res = http.get(`${BASE_URL}/balance?currency=PLN`, { headers });
    check(res, {
      "balance: status 200":    (r) => r.status === 200,
      "balance: has balance":   (r) => typeof r.json("data.balance") === "number",
      "balance: resp <100ms":   (r) => r.timings.duration < 100,
    });
  });

  sleep(0.2);

  group("Kategorie", () => {
    const res = http.get(`${BASE_URL}/categories`, { headers });
    check(res, {
      "categories: status 200": (r) => r.status === 200,
      "categories: is array":   (r) => Array.isArray(r.json("data")),
    });
  });

  sleep(0.3);

  group("Przelicznik walut", () => {
    const res = http.get(
      `${BASE_URL}/currency/convert?amount=100&from=PLN&to=EUR`,
      { headers }
    );
    check(res, {
      "currency: status 200":     (r) => r.status === 200,
      "currency: has rate":       (r) => r.json("data.exchange_rate") > 0,
      "currency: converted > 0":  (r) => r.json("data.converted_amount") > 0,
    });
  });

  sleep(randomIntBetween(1, 3));
}

export function wsTest(data) {
  const token = data.token;
  const proto  = BASE_URL.startsWith("https") ? "wss" : "ws";
  const wsUrl  = `${proto}://${BASE_URL.replace(/^https?:\/\//, "")}/ws/balance`;

  const res = ws.connect(wsUrl, { headers: { Authorization: `Bearer ${token}` } }, (socket) => {
    socket.on("open",    () => {});
    socket.on("message", (msg) => {
      wsMessagesReceived.add(1);
      const data = JSON.parse(msg);
      check(data, {
        "ws: has type":    (d) => typeof d.type === "string",
        "ws: has payload": (d) => d.payload !== undefined || d.type === "pong",
      });
    });
    socket.on("error", () => {});

    socket.setTimeout(() => { socket.close(); }, 10000);
  });

  check(res, { "ws: connected 101": (r) => r && r.status === 101 });
}

export function teardown(data) {
  console.log(`Test zakończony. Token użyty w setup: ${data.token ? "tak" : "brak"}`);
}

const vuTokenCache = {};

function getOrCreateToken(data) {
  const vuID = __VU;

  if (vuTokenCache[vuID]) return vuTokenCache[vuID];

  const email    = `vu_${vuID}_${randomString(6)}@perf.test`;
  const password = "password123";

  const start = Date.now();

  const reg = http.post(
    `${BASE_URL}/auth/register`,
    JSON.stringify({ email, password, name: `VU ${vuID}` }),
    { headers: { "Content-Type": "application/json" } }
  );

  const login = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email, password }),
    { headers: { "Content-Type": "application/json" } }
  );
  authDuration.add(Date.now() - start);

  if (login.status !== 200) {
    errorRate.add(1);
    return null;
  }

  const token = login.json("data.token");
  vuTokenCache[vuID] = token;
  return token;
}
