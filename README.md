# urlwatch

A concurrent URL health checker. It reads a list of URLs from a file,
checks them all in parallel with a bounded worker pool, and tells you
which ones are healthy, which are reachable but gated, and which are
just failing.

##

</br>
<div align="center"><a href='https://ko-fi.com/U8D024998A' target='_blank'><img height='36' style='border:0px;height:36px;' src='https://storage.ko-fi.com/cdn/kofi6.png?v=6' border='0' alt='Buy Me a Coffee at ko-fi.com' /></a></div>

## Build

```bash
go build -o urlwatch .
```

## Usage

```bash
./urlwatch -file urls.txt
```

| Flag        | Default | Description                                              |
|-------------|---------|------------------------------------------------------------|
| `-file`     | (required) | Path to the file containing URLs to check                |
| `-workers`  | `5`     | Number of concurrent workers (1–100)                      |
| `-timeout`  | `500ms` | Per-request timeout (50ms–30s), e.g. `500ms` or `2s`       |
| `-json`     | `false` | Output results as a JSON array instead of plain text       |

```bash
./urlwatch -file urls.txt -workers 10 -timeout 2s -json
```

`urls.txt` should contain one URL per line, including the scheme:

```
https://www.example.com
https://www.google.com
https://www.github.com
https://www.stackoverflow.com
https://www.wikipedia.org
https://www.reddit.com
```

Lines that aren't valid URLs get skipped with a warning at startup;
everything else in the file still runs.

Stop a run early with `Ctrl+C`. Checks already in progress finish; no new
ones start.

## Output

One line per URL, printed as results come in (order is not guaranteed):

```
https://www.wikipedia.org is reachable(403) but may require authentication
https://www.example.com is healthy(200)
https://www.google.com is healthy(200)
https://www.reddit.com is healthy(200)
https://www.github.com is healthy(200)
https://www.stackoverflow.com is reachable(403) but may require authentication
https://www.youtube.com is healthy(200)
https://www.amazon.com is healthy(200)
https://www.linkedin.com is healthy(200)
https://www.twitter.com is failure(0) with error: timeout
```

With `-json`, results are sorted by input order and include the index and
latency of each check:

```json
[{"index":0,"url":"https://www.example.com","status":"healthy","statusCode":200,"errMsg":"","latencyMs":83,"checkedAt":"2026-08-21T10:00:00Z"}]
```

## Exit codes

- `0` — every URL was healthy or reachable.
- `1` — at least one URL came back as a failure, **or** the input file
  couldn't be opened. This is meant to be checked by scripts or CI, e.g.:

  ```bash
  ./urlwatch -file urls.txt || echo "one or more services are down"
  ```

## Status categories

Each URL is classified into one of three states based on the HTTP response:

| Status       | Meaning                                          | HTTP codes           |
|--------------|---------------------------------------------------|-----------------------|
| `healthy`    | Responded successfully                             | 2xx                   |
| `reachable`  | Server responded, but access is gated              | 401, 403              |
| `failure`    | Endpoint missing, server error, or unreachable     | anything else, timeout, network error |

`reachable` isn't treated as a failure — the target is up, just
protected. `404` is, though: it means the endpoint you're watching no
longer exists, and that's exactly the kind of change a health check
exists to catch.

A `429 Too Many Requests` response is retried up to 3 times with
exponential backoff before being reported as a `failure`.

## Concurrency notes

- Worker pool size is configurable with `-workers` (default `5`, max
  `100`) to bound how many connections are open at once.
- Each check has its own timeout, configurable with `-timeout` (default
  `500ms`), independent of the others.
- Outbound requests go through an SSRF-safe HTTP client that blocks
  connections to private/internal network ranges.
