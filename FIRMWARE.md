# TRMNL X firmware — updating, recovering, debugging

Learnings from the Oct 2026 hang/update session. Read this before touching the device's firmware or
diagnosing a device that stopped checking in. Firmware source: `~/Developer/trmnl-firmware`
(upstream `usetrmnl/trmnl-firmware`); public releases: `usetrmnl/firmware`.

## Is the device alive? Check the server first

```bash
grep -E 'fw_version|last_seen' data/devices.json     # last contact + firmware it reported
grep -E 'wake|api/setup' ~/Library/Logs/trmnl-byos.log | tail
```

The device wakes every ~75 s (60 s `refresh_rate` plus wake time). If `last_seen` is stale and the prod
server is up (`make status`), the device hung or lost Wi-Fi — the server is not the problem. Server
logs record `fw=` and `batt=` on every wake.

History: the X ran **fw 1.8.2** and hung after ~5 days of continuous 75 s wakes (last wake
2026-10-20 03:42). Updating to **v1.8.17** (2026-10-25) is the attempted fix; v1.8.2→1.8.17 includes
several memory/buffer-free crash fixes and 5 GHz Wi-Fi fixes. Whether it cured the hang is not yet known —
check `last_seen` after a few days. Unreleased upstream PRs worth trying if it hangs again:
`fix/retry-cut-download`, `image-body-timeout-30s`, `perf/x-tcp-receive-window`,
`fix/reset-wifi-crash-on-5ghz`.

## Updating the firmware

- GitHub releases (`usetrmnl/firmware`) have **no attached binaries**. CI only uploads
  `trmnl-app.<tag>.bin` as a workflow artifact kept 30 days, and it builds the `trmnl` env.
- Used: the web flasher at https://trmnl.com/flash (Chrome/Edge, USB-C). Success in the web UI only means the
  flash finished, not that the device booted.
- **Do not use `scripts/flash_app.sh` for the X** — it hardcodes `--chip esp32c3` (original TRMNL). The X is
  an ESP32-S3 with its own partition table (`boards/trmnl_x_partitions.csv`, env `TRMNL_X_EPDIY`).
- OTA is possible: `/api/display` can return `update_firmware: true` plus `firmware_url`. This server
  never does; it would need a firmware-serving endpoint. The device must be awake to receive it.

## After flashing

Expect: blank/silent screen, then the device must be reset (it can sit in download mode after a flash).
USB serial shows as `/dev/cu.usbmodem*`; a hung or sleeping device prints nothing. After the reset the
screen shows "flashing modem firmware", and Wi-Fi credentials plus the saved server URL may be gone.

Recovery that worked: reset Wi-Fi, rejoin the network, and set the API server in the portal (the
"API server" field, default `https://trmnl.app`) to this server's `BASE_URL`
(`http://192.168.234.144:8080`). The device then calls `/api/setup`, and the existing record
(`device-1690`, MAC `3c0f02c31690`) and its `plugin` were kept.

The device's own menu (Settings → diagnostics) lists Firmware, Friendly ID, Wi-Fi, and **Server** —
check it first if the device is silent on our server; it may still point at trmnl.app.

### Button/touch behavior (from firmware source, `src/button.cpp`, `include/config.h`)

| Input | Effect |
|---|---|
| Hold 5–15 s, release | WiFi reset → setup portal |
| Hold ≥ 15 s | Soft reset — forgets device credentials (API key) |
| Double click | Server-assigned special function |

On the X the Wi-Fi reset prompt reads "Hold left and right corner of touch bar to reset WiFi". The
prompt is only on screen briefly, so start the hold while it shows. Exact X touch-bar gestures for the
menu were not confirmed.

## Headers the X sends (fw ≥ 1.8.3)

`Percent-Charged` (state of charge %), `Battery-Voltage`, `Battery-Charging` (**`"1"`/`"0"`**, not
`"true"`), `USB-Connected`, `Battery-Temp`, `Battery-Health`, `FW-Version`, `RSSI`, `WiFi-Band`,
`Wake-Time`, `Image-Cached`. The server stores voltage, percent, and charging on the device record.

## Device logs: `POST /api/log`

The firmware uploads `{"logs":[{...}]}` with `Access-Token`, and treats anything other than 200/204 as
failure and retries — a 404 means it re-sends the same logs every wake. `server/devlog.go` returns 204
and writes each entry to the server log as `devlog device=<id> <json>`. After a hang, grep for it:

```bash
grep devlog ~/Library/Logs/trmnl-byos.log | tail -50
```

## Battery indicator

`plugins/dashboard`: a battery icon (fill proportional to charge) + percentage is always drawn in the
header row, left of the clock, once the device has reported a level (`hasBattery()`). It uses
`Percent-Charged`, falling back to a voltage estimate (3.3–4.2 V) for firmware that sends no percentage.
It was originally shown only below 25 %; changed to always-on on 2026-10-25.

A lightning bolt is drawn left of the battery icon whenever external power is present (`onPower()`:
`Battery-Charging` is `1` **or** `USB-Connected` is `true`). Those are separate pins — power-good vs the
charger's STAT pin, which drops once the battery is full — so the bolt shows "on dock" even when full.
Each wake line logs `charging=`, `usb=` and `current=<n>mA` (fuel-gauge `Battery-Current`). The charge dock
has a flaky connection, so the bolt disappearing is the cue to reseat it. At 13:13 on 2026-10-25, with the
device reportedly on the dock, it sent `usb=false current=0mA` — unconfirmed whether the dock had lost contact
or the device doesn't report it; check by reseating the dock and watching those fields change.
