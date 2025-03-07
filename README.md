# [unfurl](https://github.com/bnnanet/unfurl)

Unfurl URLs / follow redirects. A link expander. \
(because my network blocks ads, and therefore sometimes legitimate links)

```text
http://<address>:<port>/api/unfurl
   ?url=<url>
   &depth=[10]
```

# Usage

## CLI

```sh
unfurl --depth 1 'https://tinyurl.com/2u7bcuny'
```

```text
https://unfurl.bnna.net
```

## Server

```sh
unfurld --bind 'localhost' --port '8080' --max-redirects 10
```

```sh
curl 'http://127.0.0.1:8080/api/unfurl?url=https://tinyurl.com/2u7bcuny&depth=1' |
   jq -r .result[0].url
```

```text
https://unfurl.bnna.net
```

# Build

0. Install Go

    ```sh
    curl https://webi.sh/go | sh
    source ~/.config/envman/PATH.env
    ```

1. Clone the repo

    ```sh
    git clone https://github.com/bnnanet/unfurl
    ```

2. Dev build for the your local machine:

    ```sh
    go build ./cmd/unfurld/
    ./unfurld --version

    go build ./cmd/unfurl/
    ./unfurl --version
    ```

3. Build with version info, for all targets:
    ```sh
    goreleaser release --snapshot --clean --skip=publish
    ```

Note: at least on macOS, Android only builds for `arm64`, see <https://github.com/bnnanet/unfurl/issues/1>.

# Built Live on YouTube & Twitch

This started out as a "vibe coding" session with Grok 3:

- <https://www.youtube.com/watch?v=fPVzPwm5KxE>
- <https://x.com/i/grok/share/fkPCdTh2eWdbxA8Dmte73xQrW>
