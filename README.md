# urlwatch

A concurrent URL health checker. Reads a list of URLs from a file, checks
them all in parallel using a bounded worker pool, and reports which are
healthy, which are reachable but access-gated, and which are failing.

## Build

```bash
go build -o urlwatch .
```

## Usage

```bash
./urlwatch -file urls.txt
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

Lines that aren't valid URLs are skipped with a warning printed at
startup — the rest of the file still runs normally.

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
| `failure`    | Endpoint missing, server error, or unreachable     | 404, 5xx, timeout, network error |

`reachable` is intentionally **not** treated as a failure — the target is
up and working, it's just protected. `404` **is** treated as a failure,
since it means the specific endpoint being watched no longer exists,
which is exactly the kind of change a health check should catch.

## Concurrency notes

- Worker pool size is fixed (see `numWorkers` in `main.go`) to bound how
  many connections are open at once — sized for safety against rate
  limits, not for maximum throughput.
- Each check has its own 500ms timeout, independent of the others.
