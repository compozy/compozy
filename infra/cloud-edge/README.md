# Compozy Cloud edge

The Worker routes requests to two named Cloudflare Container instances, `cloud-0`
and `cloud-1`. It checks `GET /healthz` before forwarding and tries the second
instance when the first is unready. The cron checks both instances every minute
and starts sleeping instances through the Container binding. The Go image serves
HTTP on port 8080.

Computer-key routes with a computer ID in the path use that ID for affinity. The
control-link WebSocket and browser token redemption carry the ID as
`?computer_id=<id>` for routing. The Go service still authenticates each request.
Link enrollment uses the ready pool until a computer ID exists.

The Go service receives `X-Compozy-Client-IP` from Cloudflare's
`CF-Connecting-IP` and `X-Compozy-Location` from the available city, region and
country fields, in that order. Location components are percent-encoded UTF-8
and joined by `, `. Decode with `url.PathUnescape` in Go. The Worker removes the
corresponding inbound headers when Cloudflare metadata is absent. Consumers
trust these headers only on the Worker-to-Container ingress.

`wrangler.toml` binds separate Container pools to the production and staging
custom domains. The cloud deploy workflow provisions the Docker image and
injects the Go service's runtime configuration per environment.
