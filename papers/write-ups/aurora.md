---
title: "Amazon Aurora: Design Considerations for HighThroughput Cloud-Native Relational Databases"
description: ""
tags: ["Cloud"]
reference: http://nil.csail.mit.edu/6.824/2020/papers/aurora.pdf
---

## Questions

Q. The second paragraph of Section 4.1 says "The runtime state maintained by the database lets us use single segment reads rather than quorum reads..." What runtime state does the database need to maintain in order to avoid having to read from a quorum?
