# Settmesh

## Overview

Mobile messaging app for Android (Android first, iOS considered later) with end-to-end
encryption. Private keys are generated and stored only on the user's device, never on
the server. The server acts as a blind relay that only ever sees encrypted data.

## Project identity

- **GitHub repo / organization**: `wildhoneybadger`
- **App name**: **Settmesh** — a blend of "sett" (a honey badger's burrow, an
  underground network of tunnels) and "mesh" (mesh network), reflecting the
  decentralized architecture where each user hosts their own server.
- **Visual/symbolic theme**: the honey badger — resilience, independence, protection.

## Architecture

- A single server written in **Go**, deployed via **Docker**, owned by a host and
  serving a closed community of users (no federation between servers for now).
- The mobile client is built with **React Native**, with support for configuring
  multiple different server profiles (like multiple mail accounts on the same phone).
- The server only stores: public keys, encrypted messages, encrypted files, and
  minimal metadata (who talks to whom and when — never the content).

## Encryption

- **Signal** protocol via the `libsignal` library, with React Native bindings on the
  client side.
- Each user locally generates a long-term identity key and a batch of one-time-use
  keys; only the public parts are sent to the server for the initial key exchange.
- Private keys are stored encrypted on the device, protected by a key derived from
  the user's password — distinct from the one used for server authentication.
- **Photos**: encrypted with a unique symmetric key per file. The encrypted file is
  stored on the server; the symmetric key itself is encrypted end-to-end and
  transmitted like a regular message.

## Authentication

- Anonymous accounts based solely on **username + password**, with no phone number or
  email address.
- **Invitation required to register**: open registration is disabled. Only the server
  administrator can generate invitation codes.
  - Each code is **single-use** — invalidated as soon as an account is created with
    it, even in case of partial failure.
  - Limited lifetime, e.g. **72 hours**.
  - Randomly generated each time, with no predictable link to previous codes.
  - Without a valid code, no account can be created.
- Rate-limited login attempts per IP address, with progressive delay after repeated
  failures.
- **TLS mandatory** on all client-server transport.

## Contacts

- Added only via **code or QR code** exchanged in person, with no search or address
  book import. No server-side lookup by username.

## Messages and files

- **Text messages**: end-to-end encrypted, kept on the server for a **maximum of 30
  days** before automatic deletion if undelivered.
- **Photos**: encrypted, stored separately from text, **20 MB per file** limit, **7
  day** retention period.
- No voice communication (no audio/video calls).
- Plan for a per-account quota (total size or number of pending messages) to handle
  users who stay offline for long periods.

## Aesthetics

- Interface modeled on familiar standards (Signal/Telegram-like): conversation list,
  message bubbles aligned right for the sender and left for the recipient, a compose
  field and send button fixed at the bottom of the screen.
- **Dark theme by default**, with a **gold accent color** for important elements
  (buttons, links, status indicators).

## Open source

- Code published under the **MIT** license.
- Hosted on GitHub, `wildhoneybadger` organization, `settmesh` repository.

## Getting started plan

1. Create the folder structure for the server (Go) and the client (React Native).
2. Minimal Go server responding to a simple request, containerized with Docker.
3. Add authentication (username/password + invitation system).
4. Add key exchange (Signal Protocol / libsignal).
5. Add end-to-end encrypted messaging (text).
6. Add encrypted photo sending.
7. Generate a **README.md** documenting deployment: prerequisites (Docker, Docker
   Compose), environment variables to configure, build and launch commands for the
   server container, how to generate a first admin invitation code, and how to build
   the client's Android APK.
