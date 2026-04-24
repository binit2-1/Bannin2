# Bannin — Athernex

> A three-tier application built with TypeScript and Go, consisting of a client, a server, and a background daemon.

---

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
  - [Clone the Repository](#clone-the-repository)
  - [Running the Daemon](#running-the-daemon)
  - [Running the Server](#running-the-server)
  - [Running the Client](#running-the-client)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

---

## Overview

**Bannin** is the repository that powers **Athernex** — a system composed of three cooperating services:

- A **client** (TypeScript) that provides the user-facing interface.
- A **server** (TypeScript) that acts as the application's API layer and business logic hub.
- A **daemon** (Go) that runs as a background process, handling system-level or long-running tasks independently of the main server.

---

## Architecture

```
┌──────────────┐            HTTP               ┌──────────────┐
│    Client    │ ◄───────────────────────────► │    Server    │
│ (TypeScript) │                               │ (TypeScript) │
└──────────────┘                               └──────┬───────┘
                                                      │
                                                     HTTP
                                                      │
                                               ┌──────▼───────┐
                                               │    Daemon    │
                                               │     (Go)     │
                                               └──────────────┘
```

The client communicates with the server over HTTP or WebSockets. The server delegates background or system-level work to the Go daemon, which runs as a separate long-lived process.

---

## Project Structure

```
Bannin/
├── client/       # Frontend / UI layer (TypeScript)
├── server/       # API server and business logic (TypeScript)
└── daemon/       # Background process (Go)
```

---

## Prerequisites

Make sure you have the following installed:

| Tool | Minimum Version | Purpose |
|------|----------------|---------|
| [Node.js](https://nodejs.org/) | 18+ | Run the client and server |
| [npm](https://www.npmjs.com/) or [pnpm](https://pnpm.io/) | latest | Package management |
| [Go](https://go.dev/) | 1.21+ | Build and run the daemon |
| [Git](https://git-scm.com/) | any | Clone the repository |

---

## Getting Started

### Clone the Repository

```bash
git clone https://github.com/Shreehari-Acharya/Bannin.git
cd Bannin
```

### Running the Daemon

```bash
cd daemon
go mod tidy
go run .
```

To build a production binary:

```bash
go build -o athernex-daemon .
./athernex-daemon
```

### Running the Server

```bash
cd server
npm install
npm run dev        # development mode with hot-reload
# or
npm run build && npm start   # production
```

### Running the Client

```bash
cd client
npm install
npm run dev        # starts the dev server
# or
npm run build      # production build
```

> **Tip:** Start the daemon first, then the server, then the client to ensure all services are available when the client loads.

---

## Development

Each service lives in its own directory and can be developed independently. Recommended workflow:

1. Open three terminal tabs.
2. Run the daemon (`go run .`) in one.
3. Run the server (`npm run dev`) in another.
4. Run the client (`npm run dev`) in the third.

Check each subdirectory for its own `package.json` (TypeScript services) or `go.mod` (daemon) for environment-specific scripts and dependencies.

---
