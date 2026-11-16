# Mail Pulse

A native macOS desktop app for monitoring Postfix mail servers. Load any Postfix log file and get instant visibility into delivery performance, failures, relay health, and IP reputation — no server-side setup required.

![Platform](https://img.shields.io/badge/platform-macOS-black)
![Stack](https://img.shields.io/badge/stack-Go%20%2B%20React-blue)
![Build](https://img.shields.io/badge/build-Wails%20v3-green)

---

## Features

### Overview
- **Metric cards** — Success rate, Sent, Deferred, Bounced, Errors, Warnings
- **Top Sender Domains** and **Top Recipient Domains** — scrollable, full-height tables
- Click any metric card to jump to the relevant filtered view

### Volume
- **Hourly volume chart** — Sent / Deferred / Bounced per hour (bar chart)
- **Queue Age Distribution** — how long messages spend in queue before final delivery
- **Deferred Reasons** — ranked list, click any reason to see all matching messages
- **Bounce Reasons** — ranked list, click any reason to see all matching messages
- **Top Failing Destinations** — domains with most failures, click for recipient drilldown
- **Top Failing Senders** — senders with most failures, click to see their failed messages
- **Hourly Activity Heatmap** — 24×date grid, colored by fail rate (emerald=good, amber=mixed, rose=bad)

### Insights
- **Delay Insights** — dual Y-axis trend chart (Before Qmgr / In Qmgr / Connection / Transmission)
- **Retry Timeline** — top 50 most-retried queue IDs; click any to see full delivery attempt history with timestamps, DSN codes, relay, and inter-attempt delay

### Activity
- Full delivery log — Time, Queue ID, From, To, Status, Process Time
- Filter by status (All / Sent / Deferred / Bounced)
- Full-text search across all fields

### Queue Summary
- One row per queue ID — Time, Queue ID, From, Recipient Count, Size, Status
- Expand any row to see all recipients
- Filter by status + full-text search

### Qmgr
- Queue manager activity chart — active / hold / deferred over time

### Mail Queue
- Live `mailq` output with search and refresh

### Relays
- **Throttling** — relays deferring the most messages
- **Blocking** — relays bouncing with policy/block DSN codes
- **Healthy** — relays with good delivery
- Click any relay → message drilldown with DSN + SMTP response

### Diagnostics
- **IP Reputation** — blocklist events grouped by sending IP and service (TrendMicro ERS, etc.), with per-event drilldown
- **Error Log** — all `error` and `warning` level Postfix entries, paginated

### Raw Log
- Virtual infinite-scroll viewer (handles 175k+ lines)
- Backend search (Go grep, ~200ms on 32MB)
- Live polling every 3s

---

## Architecture

```
┌─────────────────────────────────────────────────┐
│  macOS App (Wails v3)                           │
│                                                 │
│  ┌──────────────┐     ┌────────────────────┐    │
│  │  React + TS  │────▶│   Go backend       │    │
│  │  Tailwind    │     │                    │    │
│  │  Recharts    │     │  ┌──────────────┐  │    │
│  └──────────────┘     │  │  Log Parser  │  │    │
│                       │  │  In-memory   │  │    │
│                       │  │  Store       │  │    │
│                       │  │  Query layer │  │    │
│                       │  └──────────────┘  │    │
│                       └────────────────────┘    │
└─────────────────────────────────────────────────┘
```

**Go files:**
| File | Purpose |
|------|---------|
| `main.go` | Wails app entry point |
| `logservice.go` | All Wails-exported methods |
| `parser.go` | Postfix log line parser (regex-based) |
| `store.go` | In-memory log store, dedup, queue→sender map |
| `queries.go` | All analytics query methods |
| `types.go` | Shared data types |
| `mailqueue.go` | `mailq` integration |
| `parser_test.go` | 21 parser + query tests |

**Key design decisions:**
- All metrics are computed on-demand from raw parsed entries — no eager counters
- `queueFrom` map resolves sender for delivery lines (Postfix `smtp[]` lines don't carry `from=`)
- NDR/DSN messages filtered via `ndrQueues` map (queue IDs where `from=<>`)
- seenIDs dedup uses full raw log line as key
- Backend search for Raw Log (Go grep ~200ms on 32MB vs slow JS filter)

---

## Stack

| Layer | Tech |
|-------|------|
| Shell | Wails v3 (native macOS WebView) |
| Backend | Go 1.26 |
| Frontend | React 18 + Vite + TypeScript |
| Styling | Tailwind CSS v4 |
| Charts | Recharts |
| Virtual list | @tanstack/react-virtual |

---

## Build & Install

### Prerequisites
```bash
# Go 1.22+, Node 18+
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
```

### Dev build (quick iteration)
```bash
cd /Users/pankajkumar/IBM/Projects/mail-pulse
touch frontend/dist/index.html && wails3 build
# Then replace binary in installed app:
cp "bin/Mail Pulse" "/Applications/Mail Pulse.app/Contents/MacOS/Mail Pulse"
open "/Applications/Mail Pulse.app"
```

### Release build (universal DMG)
```bash
wails3 task darwin:package:universal   # builds arm64 + amd64 universal binary
wails3 task darwin:create:dmg          # packages into bin/Mail Pulse.dmg
```

Distribute `bin/Mail Pulse.dmg` — works on both Intel and Apple Silicon Macs.

### Run tests
```bash
go test ./...
```

---

## Log files

Place Postfix `.log` files in `~/.mail-pulse/logs/` — the app lists them all in the file picker.

Tested with:
- `mail.log` — 32MB, 175k lines (multiple rotated files merged)
- Individual rotated archives (`mail.0.log` – `mail.5.log`)

---

## Known patterns in logs

| Pattern | Root cause |
|---------|-----------|
| ~4,400 defers with DSN `4.7.500` | Microsoft 365 throttling |
| ~4,000 defers to Cloudflare-proxied domains | IPv6 connectivity issues |
| `5.2.1` bounces | Inactive recipient accounts |
| TrendMicro ERS-DUL blocks | Sending IP on TrendMicro blocklist |
