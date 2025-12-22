---
title: "Don’t Settle for Eventual:Scalable Causal Consistency for Wide-Area Storage with COPS"
description: ""
tags: ["Scalability","Causal Consistency", "Wide-Area Storage"]
reference: http://nil.csail.mit.edu/6.824/2020/papers/cops.pdf
---

## Novelty

## Key Takeaways

---

## Details



---

## Questions

Q. The last sentence in Section 4.3 says a client clears its context after a put, replacing the context with just the put. The text observes "This put depends on all previous key-version pairs and thus is nearer than them." Why does clearing the context and replacing it with just the put make sense? You might think that the client's subsequent puts would need to carry along the dependency information about previous gets. What entity ultimately uses the context information, and why does it not need the information about gets before the last put?
