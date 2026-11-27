# Respond to hosted tunnel exhaustion

Review capacity at 700 allocated tunnels and page at 900. Compare the cloud gauge with Cloudflare's inventory for the same environment. Look for stuck `provisioning` or `deleting` rows and API rate limits. Verify that the normal sweeper is running. A sweep can delete failed or expired generations, tunnels for unlinked or suspended accounts and tunnels offline for 14 days.

Use the admin console to inspect a hostname or mark it for removal through the generation fence. Check ownership and current generation before touching a DNS record or named tunnel. If demand still approaches the Cloudflare account limit, coordinate capacity with Cloudflare before accepting more hosted routes. Record resulting errors and update the public incident.
