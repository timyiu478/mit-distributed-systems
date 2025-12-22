# Sharded Key/Value Service

## High Level Architecture

![](assets/sharded_kv_hl_architecture.png)

## Main Challenges

The main challenges in this lab will be ensuring linearizability of Get/Put operations while handling 1) changes in the assignment of shards to shardgrps, and 2) recovering from a controller that fails or is partitioned during ChangeConfigTo.

1. ChangeConfigTo moves shards from one shardgrp to another. A risk is that some clients might use the old shardgrp while other clients use the new shardgrp, which could break linearizability. You will need to ensure that at most one shardgrp is serving requests for each shard at any one time.
1. If ChangeConfigTo fails while reconfiguring, some shards may be inaccessible if they have started but not completed moving from one shardgrp to another. To make forward progress, the tester starts a new controller, and your job is to ensure that the new one completes the reconfiguration that the previous controller started.

## Known Issues

Test linearizability with groups joining/leaving and 1 concurrent clerks put/get's:

* history is not linearizable

Test linearizability with groups joining/leaving and 1 concurrent clerks put/get's:

* timeout storm on a specific replica group (servers named server-7-0, server-7-1, server-7-2)

```
2025/12/22 16:36:10 ck 9: unable to get Put response from leader idx 1 within timeout, servername is server-7-1
2025/12/22 16:36:10 ck 9: unable to get Put response from leader idx 2 within timeout, servername is server-7-2
2025/12/22 16:36:10 ck 9: unable to get Put response from leader idx 0 within timeout, servername is server-7-0
2025/12/22 16:36:10 ck 9: unable to get Put response from leader idx 1 within timeout, servername is server-7-1
2025/12/22 16:36:10 ck 9: unable to get Put response from leader idx 2 within timeout, servername is server-7-2
2025/12/22 16:36:10 ck 9: unable to get Put response from leader idx 0 within timeout, servername is server-7-0
2025/12/22 16:36:10 ck 9: unable to get Put response from leader idx 1 within timeout, servername is server-7-1
2025/12/22 16:36:10 ck 9: unable to get Put response from leader idx 2 within timeout, servername is server-7-2
2025/12/22 16:36:10 ck 9: unable to get Put response from leader idx 0 within timeout, servername is server-7-0
2025/12/22 16:36:10 ck 9: unable to get Put response from leader idx 1 within timeout, servername is server-7-1
2025/12/22 16:36:10 ck 9: unable to get Put response from leader idx 2 within timeout, servername is server-7-2
2025/12/22 16:36:10 ck 9: unable to get Put response from leader idx 0 within timeout, servername is server-7-0
2025/12/22 16:36:11 ck 9: unable to get Put response from leader idx 1 within timeout, servername is server-7-1
2025/12/22 16:36:11 ck 9: unable to get Put response from leader idx 2 within timeout, servername is server-7-2
2025/12/22 16:36:11 ck 9: unable to get Put response from leader idx 0 within timeout, servername is server-7-0
```
