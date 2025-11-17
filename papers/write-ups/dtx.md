---
title: "Principles of Computer System Design An Introduction - Chapter 9"
description: ""
tags: ["Distributed Transaction", "Atomicity", "Two-Phase Locking"]
reference: https://ocw.mit.edu/courses/res-6-004-principles-of-computer-system-design-an-introduction-spring-2009/resources/atomicity_open_5_0/
---

## 9.1.5 Before-or-After Atomicity: Coordinating Concurrent Threads

### Concurrency coordination requirements

* Sequence coordination: Action W must happen before action X
* Before-or-After atomicity: Concurrent actions have the before-or-after property if their **effect** from the point of view of their **invokers** is the same as if the actions occurred either **completely before** or **completely after** onanother.

* Sequence coordination vs Before-or-After atomicity:
    * The before-or-after atomicity property does not necessarily know the **identities** of all the other actions that might touch the shared variable.

## 9.1.6 Correctness and Serialization

Goal: **Application Independence**. We want to be able to make an argument for correctness of the mechanism that provides before-or-after atomicity without getting into the question of whether or not the application using the mechanism is correct. 

Correctness concept: coordniation among concurrent actions can be considered to be correct if **every result** is guaranteed to be one that could have been obtained by some **purely serial application** of those same actions.
    * purely serial application: the application that running with 1 thread

The before-or-after atomicity has the effect of serializing the actions, so it follows that before-or-after atomicity guarantees correctness of coordination.

**External Time Consistency**: if there is any external evidence (such as a printed receipt) that before-or-after action T1 ended before before-or-after action T2 began, the serialization order of T1 and T2 inside the system should be that T1 precedes T2.

**Sequential Consistency**: when the processor concurrently performs multiple instructions from the same instruction stream, the result should be as if the instructions were executed in the original order specified by the programmer.

## 9.5.2 Simple Locking



## 9.5.3 Two-Phrase Locking



## 9.6.3 Multiple-Site Atomicity: Distributed Two-Phase Commit



---

## Questions

Q. What the applications require external time consistency or sequential consistency?

Q. 6.033 Book. Read just these parts of Chapter 9: 9.1.5, 9.1.6, 9.5.2, 9.5.3, 9.6.3. The last two sections (on two-phase locking and distributed two-phase commit) are the most important. The Question: describe a situation where Two-Phase Locking yields higher performance than Simple Locking.
