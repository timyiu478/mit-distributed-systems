---
title: "ZooKeeper: Wait-free coordination for Internet-scale systems"
description: ""
tags: ["Coordination Primitive", "Distributed Lock", "Configuration Management"]
reference: https://pdos.csail.mit.edu/6.824/papers/zookeeper.pdf
---

## Novelty

## Takeaways

---

## Questions

Q. One use of Zookeeper is as a fault-tolerant lock service (see the section "Simple locks" on page 6). Why isn't possible for two clients to acquire the same lock? In particular, how does Zookeeper decide if a client has failed and it can give the client's locks to other clients?
