# blackit-tz

Time zone lookup from coordinates, with no API key and no outbound calls at
runtime.

```
GET /timezone?lat=47.4979&lon=19.0402&t=1782000000
→ {"timeZone":"Europe/Budapest","offsetMinutes":120,"abbreviation":"CEST","timestamp":1782000000}

GET /timezone/health
→ {"status":"ok","probe":"Europe/Budapest"}
```

`t` is a unix timestamp in seconds and may be omitted — it defaults to now. It
matters: the same place has a different offset in January and in July.

## Why it exists

[`celestial-chart`](https://github.com/Fekete85/celestial-chart) needs the UTC
offset of the observing site for its settings form. A browser cannot work that
out for an arbitrary point on Earth — only for the viewer's own zone — so it
takes an outside source.

Upstream `d3-celestial` solved this by baking the author's TimeZoneDB account id
into the library, which meant every page embedding it sent its visitors'
coordinates to a third party, unasked, on a shared quota.

This service gives the same answer without a key. The timezone polygons
([`ringsaturn/tzf`](https://github.com/ringsaturn/tzf)) are compiled into the
binary and the IANA database comes from Go's `time/tzdata`, so **nothing is
requested from anywhere at runtime**. It holds no secret, writes nothing and
keeps no state.

What a longitude estimate (15° per hour) cannot do, this can:

| place | estimate | actual |
|---|---|---|
| Delhi | +300 | **+330** (UTC+5:30) |
| Budapest, July | +60 | **+120** (CEST) |
| Budapest, January | +60 | **+60** (CET) |

Oceans are covered too, by the nautical `Etc/GMT±N` zones.

## Running it

```bash
docker compose up -d --build
curl -s localhost:8080/timezone/health
```

| variable | default |
|---|---|
| `PORT` | `8080` |
| `ALLOWED_ORIGINS` | `https://celestial.blackit.hu,https://csillag.blackit.hu` |

`ALLOWED_ORIGINS` is a comma-separated CORS allow-list. A foreign origin gets no
`Access-Control-Allow-Origin` header at all. `*` lets anyone call it — there is
no authentication and no cookie involved, so that is defensible, but it should
be a deliberate choice rather than a default.

## Deployment

It runs behind Traefik on `api.blackit.hu`, split off by path with
`PathPrefix(/timezone)` at `priority=100`.

It is deliberately **not** folded into the existing `blackit-api` service on the
same hostname: that one holds a Zammad API token and sits on the internal Zammad
network. Putting a public, permissive-CORS endpoint in the same process would
place a bug in the new handler next to the ticketing system's credential. The
router split gives the same hostname without giving up the isolation.

## Licence

MIT. See [`LICENSE`](LICENSE).
