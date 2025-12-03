---
title: "Scaling Memcache at Facebook"
description: ""
tags: ["Cache"]
reference: https://pdos.csail.mit.edu/6.824/papers/memcache-fb.pdf
---

## Novelty

## Design Considerations

Access Patterns:

* Read >>>> Write

---

## Questions

Q. Section 3.3 implies that a client that writes data does not delete the corresponding key from the Gutter servers, even though the client does try to delete the key from the ordinary Memcached servers (Figure 1). Explain why it would be a bad idea for writing clients to delete keys from Gutter servers.
