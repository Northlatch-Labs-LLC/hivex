# Start the Hivex Suite — a plain-English guide

You need no technical background for this. The whole product runs on your Mac.

## What you are starting

The Hivex product has three parts, and one command starts all of them:

1. **The customer website** — the public storefront where customers read about the product, see pricing, and sign up. It also contains the **customer portal**: accounts, API keys, usage, and checkout.
2. **The Hivex Harness office** — your private control room where the agents and the team wiki live.
3. **The Gateway** — the engine behind the scenes. It runs as part of the customer website; you never open it separately.

## How to start (one step)

1. Open the **Terminal** app (press `Cmd + Space`, type `Terminal`, press Enter).
2. Copy and paste this line, then press Enter:

   ```
   cd /Users/admin/Desktop/hivebot-main && ./start-suite.sh
   ```

3. Wait about ten seconds. You will see two links printed. Click them (or type them into your browser):
   - Customer website + portal: **http://localhost:20127/landing**
   - Harness office: **http://localhost:7911**

Keep the Terminal window open while using the suite. Minimizing is fine; closing the window shuts it down.

## How to stop

Paste this into the same Terminal window:

```
cd /Users/admin/Desktop/hivebot-main && ./stop-suite.sh
```

## How to sign in

- **Customer portal (admin):** email `ops@northlatchlabs.dev`, password `REDACTED-rotate-via-operator`. After signing in, the admin pages live at the Admin section of the portal.
- **Customer portal (customers):** they use the Sign up page to create their own account.
- **Gateway operator dashboard:** at http://localhost:20127/login — password `123456`. This is the technical control room for the Gateway; you rarely need it.

## If something does not work

- **A link will not open:** give it ten more seconds — the website takes a moment to wake up the first time each day.
- **Still nothing:** in the project folder, open the `logs` folder. The two files inside (`gateway.log` and `harness.log`) say what happened. Send those files to your engineer.
- **"Port already in use" message:** the suite is probably already running. Try the links first; if they work, you are done. If not, run the stop command above once, wait five seconds, then start again.

## What "localhost" means

These links only work on your Mac — they are private to you, nothing is published to the internet. Customers will only reach the storefront after the product is deployed to a public address, which is a separate step (ask your engineer when you are ready).
