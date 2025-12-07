## Challenges

* Server communicate the log index start from 0
* Each server has its own snapshot, lastIncludedIndex can be different
* How to handle len(rf.Log) = 0
* How InstallSnapshot should interact with the state and rules in Figure 2
