# Auth Web Spec

Implement a login page that authenticates users and redirects back with a token.

---

## Flow

```
Browser opens:  <your_login_url>?port=54321
User logs in:   (your auth: form, OAuth, SSO, etc.)
Redirect to:    http://localhost:54321/callback?token=<client_token>
```

---

## What to implement

### 1. Accept `?port=<port>` query parameter

The login page URL will be called with a `port` query parameter. Preserve this value through your entire auth flow.

### 2. Authenticate the user

Use whatever auth method you need (login form, OAuth, SSO). Map the authenticated user to their client token.

### 3. Redirect to localhost

After successful auth, redirect to:

```
http://localhost:<port>/callback?token=<client_token>
```

- `port` — from the original query parameter
- `token` — a valid client token (must work with `/projects` endpoint on your webhook server)
- **Always redirect to `http://localhost:<port>/callback`** — never accept a custom callback URL

---

## Minimal example

```python
from fastapi import FastAPI, Query
from fastapi.responses import RedirectResponse

app = FastAPI()

@app.get("/login")
def login(port: int = Query()):
    token = "tok_alice_abc123"  # look up from your auth
    return RedirectResponse(f"http://localhost:{port}/callback?token={token}")
```
