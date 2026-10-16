// Minimal client for the URL shortener API. No libraries, no build step.

const $ = (id) => document.getElementById(id);
const output = $("output");

// api sends a request and returns { status, ok, data }.
async function api(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  // 204 No Content has no body to parse.
  const data = res.status === 204 ? null : await res.json();
  return { status: res.status, ok: res.ok, data };
}

// show prints a result. textContent (never innerHTML) keeps server data from
// being interpreted as HTML.
function show({ status, ok, data }) {
  output.className = ok ? "" : "error";
  output.textContent = `HTTP ${status}\n\n` + (data ? JSON.stringify(data, null, 2) : "(no content)");
}

function showFailure(err) {
  output.className = "error";
  output.textContent = `Request failed: ${err.message}`;
}

// codePath builds /shorten/{code}... from the code box, escaping it safely.
function codePath(suffix = "") {
  const code = $("code").value.trim();
  if (!code) throw new Error("enter a short code first");
  return `/shorten/${encodeURIComponent(code)}${suffix}`;
}

$("create-form").addEventListener("submit", async (event) => {
  event.preventDefault(); // stop the browser's own form submission
  try {
    const result = await api("POST", "/shorten", { url: $("create-url").value });
    show(result);
    if (result.ok) {
      const link = `${location.origin}/${result.data.shortCode}`;
      $("short-link-a").href = link;
      $("short-link-a").textContent = link;
      $("short-link").hidden = false;
      $("code").value = result.data.shortCode;
    }
  } catch (err) {
    showFailure(err);
  }
});

$("stats-btn").addEventListener("click", async () => {
  try {
    show(await api("GET", codePath("/stats")));
  } catch (err) {
    showFailure(err);
  }
});

$("delete-btn").addEventListener("click", async () => {
  try {
    show(await api("DELETE", codePath()));
  } catch (err) {
    showFailure(err);
  }
});

$("update-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    show(await api("PUT", codePath(), { url: $("update-url").value }));
  } catch (err) {
    showFailure(err);
  }
});
