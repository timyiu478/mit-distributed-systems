---
title: "Scaling Memcache at Facebook"
description: ""
tags: ["Cache"]
reference: https://pdos.csail.mit.edu/6.824/papers/memcache-fb.pdf
---

## Novelty

## Design Considerations

* Access Pattern: Read >>>> Write
* Read from multiple data sources: backend servers, MySQL DB, HDFS installations
* Limited engineering resources and time

---

## Details

### Reducing Latency of memcache response



---

## Questions

Q. Regarding to query cache, why the web server issues SQL statements to the database and then sends a **delete** request to memcache that invalidates any stale data insteal of "update"?

Q. Section 3.3 implies that a client that writes data does not delete the corresponding key from the Gutter servers, even though the client does try to delete the key from the ordinary Memcached servers (Figure 1). Explain why it would be a bad idea for writing clients to delete keys from Gutter servers.

