import http from "k6/http";
import { sleep, check } from "k6";
import { uuidv4 } from "https://jslib.k6.io/k6-utils/1.4.0/index.js";

// --- config ---
const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
const API_KEY = __ENV.API_KEY || "";

const REPOS = [
  "torvalds/linux",
  "golang/go",
  "microsoft/vscode",
  "facebook/react",
  "vuejs/vue",
];

export const options = {
  stages: [
    { duration: "10s", target: 5 },
    { duration: "30s", target: 10 },
    { duration: "10s", target: 0 },
  ],
  thresholds: {
    http_req_duration: ["p(95)<2000"],
  },
};

const headers = {
  "Content-Type": "application/json",
  ...(API_KEY && { "X-API-Key": API_KEY }),
};


function subscribe() {
  const email = `user_${uuidv4().slice(0, 8)}@test.com`;
  const repo = REPOS[Math.floor(Math.random() * REPOS.length)];

  const res = http.post(
    `${BASE_URL}/api/subscribe`,
    JSON.stringify({ email, repo }),
    { headers },
  );

  check(res, { "subscribe 200": (r) => r.status === 200 });
}

function confirmFake() {
  const fakeToken = uuidv4();
  const res = http.get(`${BASE_URL}/api/confirm/${fakeToken}`);

  check(res, {
    "confirm fake 400/404": (r) => r.status === 400 || r.status === 404,
  });
}

function getSubscriptions() {
  const res = http.get(`${BASE_URL}/api/subscriptions/?email=user@test.com`, {
    headers,
  });

  check(res, { "subscriptions 200": (r) => r.status === 200 });
}

function unsubscribeFake() {
  const fakeToken = uuidv4();
  const res = http.get(`${BASE_URL}/api/unsubscribe/${fakeToken}`);

  check(res, {
    "unsubscribe fake 400/404": (r) => r.status === 400 || r.status === 404,
  });
}

export default function () {
  const roll = Math.random();

  if (roll < 0.4) {
    subscribe();
  } else if (roll < 0.6) {
    confirmFake();
  } else if (roll < 0.8) {
    getSubscriptions();
  } else {
    unsubscribeFake();
  }

  sleep(0.5);
}
