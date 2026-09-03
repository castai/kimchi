# Coordination board — review, fix, and validation report (2026-09-02)

Branch: `feat-agent-comms-imprv` · uncommitted working tree (feature not yet committed).
This document is the canonical, permanent record of the 2026-09-02 review round, the fixes
applied (findings A–D), and the live-testing evidence for the subagent communication +
coordination board feature. Transient working copies of related research notes live in
`.kimchi/docs/`.

---

## 1. Scope: the "coordination internal board" requirement

The feature requirement (Ferment objective): **an internal coordination board where subagents
in a group can post notes/work/findings/warnings, readable by the group, with governed safety
properties** (motivated by the July 2026 OpenAI agent-swarm incident — see Sources).

Requirement verdict: **MET** — the board ships as:

- `src/extensions/agents/manager/board.ts` — `BoardStore`: host-stamped authorship, append-only,
  same-group scoping, per-board cap 200 / global cap 2048 (FIFO), 120s dedupe (own namespace),
  `cleanupRoot`, body-free `subagents:board` events (`posted`, `evicted`).
- `src/extensions/agents/message-tool.ts` — `post_agent_note` / `read_agent_board`
  (title ≤120, body ≤2048, since_id/kind/limit ≤200).
- `src/extensions/agents/manager/agent-manager.ts` — `postBoardEntry`, `readBoardEntries`,
  `getBoardSummary(ies)ForRoot`, board event fan-out, board hint in `list_agent_contacts`.
- `src/extensions/agents/index.ts` — two-layer coordinator digest: one-shot initial snapshot in
  `before_agent_start` (guarded against re-append) + live feed via
  `pi.sendMessage(..., { deliverAs: "followUp", triggerTurn: true })` per board event.
- `src/extensions/agents/prompt/prompts.ts` — worker + coordinator prompt contract
  (data-not-instructions, no-secrets, append-only).
- `docs/subagentComms/subagent-communication-protocol.md` §10 — board protocol.

## 2. Review findings (2026-09-02) and their resolution

Report (transient copy): `.kimchi/docs/review-agent-comms-board-2026-09-02.md`.

### Must-fix (Comment severity)

| # | Finding | Resolution | Anchor (verified 2026-09-02) |
|---|---|---|---|
| A | Global-ceiling eviction spliced the oldest entry but never set `evicted`, so no body-free `{action:"evicted"}` event fired (protocol §10.3/10.6 violated). | `BoardStore.post()` sets `evicted = oldest` after the global splice when per-board eviction didn't already set it. | `board.ts:216` — `if (!evicted) evicted = oldest`. Regression test `emits global-evicted event when posting past 2048 entries across multiple boards` passes. |
| B1 | `cleanupRoot` had no callers; dead roots' boards/dedupe keys leaked. | Wired into `AgentManager.disableCommunication`: cleans the explicit root, or the bound root when called without args. | `agent-manager.ts:834-835`. Regression test `cleanupRoot after disableCommunication clears board entries — repost is not deduped` passes. |
| B2 (nit 6) | Dedupe keys built from pre-truncation values; `cleanupRoot` reconstructed from truncated ones → key mismatch leaked. | Dedupe key now built from EFFECTIVE (normalized+truncated) values; comment documents the invariant. | `board.ts:152-153`. |
| C | Digest guard needle `"## Coordination board digest"` also matched static prose in `COORDINATOR_MESSAGE_PROMPT` → composed prompts never received the live digest. | Guard now matches the dynamic-only pattern `"\n## Coordination board digest\n### Coordination board"` (static prose has backticks + no group heading). | `index.ts:825` (guard), `index.ts:832` (digest build from `getBoardSummariesForRoot`). |
| D-major (finding 4) | Tool schema `maxLength` rejects over-limit title/body before store truncation → `truncated` receipt unreachable via the tool. | **Accepted by design** (defense-in-depth): schema rejects with `invalid_schema`; store truncation only reachable by direct host-API calls. Documented in protocol §10.5. | `protocol.md:296`. |

### Nits (all resolved)

- `agent-manager.ts` `postBoardEntry`: duplicated return branches collapsed (single return).
- Typo `list_gent_contacts` → `list_agent_contacts` (`protocol.md:210`).
- `fedBoardEntries` (live-feed dedupe set) cleared on all three teardown paths:
  `index.ts:1187` (unbind branch), `index.ts:1241` (`session_before_switch`),
  `index.ts:1306` (`session_shutdown`).

## 3. Live-testing evidence (2026-09-02/03, this working tree)

| Check | Result |
|---|---|
| Agents unit slice `vitest run --dir src src/extensions/agents` | **380/380 passed** (29 files) — repeated ×2 (after final lint fix) |
| Named regression tests | `emits global-evicted event...` 1 passed; `cleanupRoot after disableCommunication...` 1 passed |
| Full unit suite `pnpm run test` | **9093 passed / 4 failed / 13 skipped (9110)** — the 4 failures are environment-dependent and in files untouched by this diff: `rtk-rewrite.test.ts detectRtk` ×3 (rtk binary not on PATH) + `modes/acp/server.test.ts` vision-model cache test ×1 |
| Typecheck `pnpm run typecheck` | **clean** (`tsc --noEmit`, no diagnostics) |
| Lint `pnpm run lint` | **clean** (biome, 1223 files, 0 warnings; one Builder-introduced dead `now`/`now++` pair in `agent-manager.test.ts` was found by re-lint and removed, re-verified clean) |
| FULL TUI e2e `pnpm run test:e2e:tui` | **EXIT=0; count corrected in §3.3 → 100 passed + 2 skipped, not 102/102** — durable log: `.kimchi/docs/tui-e2e-full-run-2026-09-02.log` (396 lines; 100 ✔ lines, 2 `-` skips at lines 108 & 128, `EXIT=0` at line 396). Binaries rebuilt before the run (fixture `fake-openai-server.ts` changed). |

All validation runs (2026-09-02/03) postdate the only branch merge (2026-08-27), so no stale
evidence. Working-tree diff at validation time: 13 files, +1591/−41 vs HEAD.

## 3.1 Review round 2 (2026-09-03): regression found and fixed

A fresh re-review of the post-fix tree (post findings A–D above) verified all four original findings
VERIFIED-FIXED but caught **one real regression introduced by fix B**, plus four nits. All were fixed
the same round and re-validated:

| # | Finding | Resolution | Anchor |
|---|---|---|---|
| R2-1 (must) | `disableCommunication` overwrote `this.communicationRootSessionId` BEFORE computing `cleanupRoot = rootSessionId ?? this.communicationRootSessionId` → the documented no-arg path resolved to `undefined` and leaked the bound root's board state (dead fallback). | Compute `cleanupRoot` immediately after the early-return guard, before any mutation; cleanup call stays at the end. Added the missing no-arg regression test `disableCommunication() no-arg cleans up the currently-bound root — repost is not deduped`. | `agent-manager.ts:828-835`; test at `agent-manager.test.ts` (:3163-3192) |
| R2-2 (nit) | `cleanupRoot` hand-duplicated the dedupe-key format. | Uses `createDedupeKey(...)` — byte-identical (normalize is idempotent on stored effective values), kills format-drift risk. | `board.ts:317` |
| R2-3 (nit) | Tool schema `maxLength` literals `120`/`2048`. | Uses exported `BOARD_ENTRY_TITLE_MAX`/`BOARD_ENTRY_BODY_MAX`. | `message-tool.ts:4,55,61` |
| R2-4 (nit) | Protocol doc referenced a phantom `board.test.ts`. | Points at the real coverage (`manager/agent-manager.test.ts` + `message-tool.test.ts`). | `protocol.md:321` |
| R2-5 (nit) | `evicted` event attributed the poster's root/group; doc wording implied two events per post. | Event now carries the EVICTED entry's `rootSessionId`/`groupId`; doc §10.3 clarified to one `evicted` event per post attributed to the evicted entry's own board. | `agent-manager.ts:919-920`; `protocol.md` §10.3 |

Round-2 live validation (this working tree): agents slice **381/381** (29 files, incl. the new no-arg
test; the two named `disableCommunication` tests pass 2/2) · full unit suite **9094 passed / 4 failed /
13 skipped (9111)** — the 4 failures are unchanged environment-dependent cases in untouched files
(`detectRtk` ×3, acp vision-model cache ×1) · typecheck clean · lint clean (2 import/format diagnostics
the fixer introduced were auto-fixed and re-verified 0 warnings) · FULL TUI e2e re-run against rebuilt
binaries: **EXIT=0; count corrected in §3.3 → 100 passed + 2 skipped, not 102/102** — durable log `.kimchi/docs/tui-e2e-round2-2026-09-03.log` (skips at lines 72 & 99, `EXIT=0`).

## 3.2 Final acceptance review (round 3, 2026-09-03)

An independent final review pass over the round-2-fixed tree confirmed the review/fix loop is closed:

**Verdict: APPROVED — no real review points. No findings.**

| Anchor | Status |
|---|---|
| R2-1 `disableCommunication` cleanupRoot ordering (`agent-manager.ts:823-835`, compute-before-mutate at :827, both paths converge at :834) | VERIFIED-FIXED |
| R2-1 no-arg regression test (`agent-manager.test.ts:3163-3192`; named run `-t "disableCommunication"` → **2/2 pass**) | VERIFIED-FIXED |
| R2-2 `createDedupeKey` reuse (`board.ts:317`; byte-identical for stored effective values — `normalize` idempotent, `board.ts:86`) | VERIFIED-FIXED |
| R2-3 schema constants (`message-tool.ts:4-6`, `BOARD_ENTRY_TITLE_MAX=120`, `BOARD_ENTRY_BODY_MAX=2048`; schema effect unchanged) | VERIFIED-FIXED |
| R2-4 phantom `board.test.ts` reference removed (`protocol.md`; real coverage referenced) | VERIFIED-FIXED |
| R2-5 evicted event attribution (`agent-manager.ts:915-921` → `result.evicted.rootSessionId/groupId`, set at `board.ts:201/:216`; tests `:2851` + `:3048`) | VERIFIED-FIXED |
| Round-1 anchors still hold: `board.ts:216`, `board.ts:152-153`, `agent-manager.ts:823-835`, `index.ts:825/:832`, `index.ts:1187/:1241/:1306` | VERIFIED-FIXED |

Round-3 sweep also ran clean: typecheck 0 diagnostics, agents slice 381/381, lint clean (1223 files).
Transient review file: `.kimchi/docs/review.md`.

## 3.3 Independent re-validation (round 4, 2026-09-03) — full-suite claim corrected

An independent re-validation session re-ran every gate on this working tree and found the feature
validated clean, with ONE correction to the full-suite claim, plus one latent test defect fixed:

- Agents unit slice `vitest run --dir src src/extensions/agents` → **381/381** (29 files) — reproduced.
- Typecheck clean (0 diagnostics), lint clean (1223 files) — reproduced.
- Focused TUI e2e `agent-communication` (the feature's own scenarios) → **2/2 pass** — reproduced
  (`communicating child asks through parent…` + `two same-batch workers post and read via the
  coordination board`).
- **Correction — the "102/102 scenarios, EXIT=0" claims in §3/§3.1 were actually 100 passed + 2
  SKIPPED.** Both durable logs show two skipped scenarios, not zero: `tui-e2e-full-run-2026-09-02.log`
  (skip lines 108 `clipboard-wayland-idle` and 128 `dap-debug-workflow` happy path, 100 ✔ lines, EXIT=0 at
  line 396) and `tui-e2e-round2-2026-09-03.log` (same two skips at lines 72 & 99, 100 ✔ lines
  (`grep -c ✔` = 100), EXIT=0). The suite exited 0 in both runs with 2 skipped, so the feature evidence
  was valid, but the round counts were overstated.
- Today the skipped test ACTIVATES: this shell exports `JS_DEBUG_PATH` pointing at a js-debug install
  (extracted 2026-09-02), which flips the test's load-time skip guard to run → it fails. Root cause is
  two PRE-EXISTING defects, neither caused by this diff (git diff HEAD touches zero dap files):
  1. **Test self-contradiction** (fixed): the test body set `KIMCHI_DAP_BINARIES: ""`, which under the
     override semantics (`adapters.ts:206-215`) force-disables ALL adapters including js-debug, so the
     "happy path" could never pass even on a machine with a working install. Fixed in
     `tests/e2e/tui/dap-debug-workflow.test.ts` by whitelisting `"js-debug"` (matches the test's own
     intent: keep other machine adapters inert). Detection now works (`DAP: js-debug` footer active).
  2. **Pre-existing js-debug TCP adapter defect** (FIXED in round 5, 2026-09-03 — see §3.4): even with detection
     working and with ABSOLUTE source paths, breakpoints never bind (`hit: false`) and writes EPIPE.
     Reproduced independently by the DAP extension's own integration suite
     `src/extensions/dap/integration.test.ts` → **3 failed** (`debug_state_at captures locals` =
     `hit` false at absolute `fixturePath`; `terminates session` = `write EPIPE`; `debug_trace_calls` =
     30s timeout). This is a DAP-extension (#1051) environment/integration issue that predates and is
     orthogonal to the subagent-comms board work; it only surfaces because the machine now has a
     js-debug install on `JS_DEBUG_PATH`.

Round-4 verdict: the coordination-board feature and its fix loop remain **validated** (all feature
slots + feature e2e green). Full TUI suite re-run today: **1 failed of 102 scenarios (dap-happy), every
other scenario did not fail** — EXIT=1 solely due to that env-activated, pre-existing DAP test.

Round-5 update (2026-09-03): the dap-happy failure is no longer failing — see §3.4 for the fix and the fresh evidence (integration 3 passed, full DAP suite 179 passed | 4 skipped, e2e dap-debug-workflow 2/2 incl. dap-happy).

## 3.4 js-debug TCP adapter defect — FIXED (round 5, 2026-09-03)

Root causes (verified by raw-DAP hand-driven probes against `node $JS_DEBUG_PATH 0 127.0.0.1`, js-debug
v1.117, Node v22.22.2 — `/tmp/dap-probe2/3/4.mjs`):

1. **Provisional breakpoints on the manager connection.** js-debug's `dapDebugServer.js` is
   manager→child: the launch response is deferred until root `configurationDone`; only then does it send
   the `startDebugging` reverse-request. Caller's pre-child `setBreakpoints` landed on the ROOT connection
   as provisional (`{"verified":false}`); `startChildSession` sent child `configurationDone` immediately
   after `initialized` → debuggee ran with zero breakpoints → `hit:false`, then write EPIPE / 30s
   timeouts. **Fix** (`types.ts`, `client.ts`): new `DapConfigForward` + `childConfigForwards` map on
   `DapClient`; `sendRequest` records `setBreakpoints`/`setExceptionBreakpoints`, and `startChildSession`
   replays them on the child after its `initialized`, before its `configurationDone`.
2. **Frame-less `evaluate` rejected.** The child's `evaluate` with `{context:"repl"}` and no `frameId`
   returns opaque `success:false` (empty message → "DAP evaluate failed: unknown error"); with the
   stopped frame's id it works. **Fix** (`composed.ts`): `debugStateAt` passes `backtrace[0]?.id`;
   `evaluateString` fetches `session.getStackFrame()` and passes `frames[0]?.id` (`!= null` guards —
   js-debug's frame id 0 is falsy).
3. **Dead-client reuse race** (test 2 `write EPIPE` / ECONNRESET): after test 1's `terminate()` SIGKILLs
   the shared adapter, `getOrCreate` could return the dead client (TCP wrapper's `exitCode` is always
   null, so liveness is undetectable that way). **Fix**: `getOrCreate` drops `existing.terminated` clients
   and respawns.
4. **Mid-execution stop truncated stdout** (`result.stdout` missing `result=`): the first breakpoint
   hit is typically inside a loop, so output captured at the stop misses everything printed afterward.
   **Fix** (`composed.ts`): `debugStateAt` loops `session.continue()` until the terminated rejection
   (`isTerminatedError`), but ONLY for ephemeral sessions (`shouldTerminate`) — interactive sessions are
   not run to completion. Same loop added to `debugTraceCalls`, whose single `continue()` otherwise
   resolved early with the program never finishing → 0 calls.
5. **Root-vs-child terminated race** (`debug_trace_calls` 0 calls after #4): the root manager connection
   also emits `terminated`, and it can resolve run-to-completion waiters BEFORE the child connection
   streams the debuggee's final `output` frames (instrumentation yielding the event loop masked the race).
   **Fix** (`client.ts` message reader): drop root-connection `terminated` when `client.childClient`
   exists; the child's in-order terminated (after its own output) is authoritative.

Round-5 verification results:

- Integration suite `src/extensions/dap/integration.test.ts` → **3 passed | 4 skipped (7)** in ~1.5s; the
  4 skips are absent dlv/debugpy adapters by design.
- `debug_trace_calls` reran **3/3 consecutive passes** post-fix (no instrumentation).
- Full DAP unit suite `vitest run --dir src src/extensions/dap` → **8 files, 179 passed | 4 skipped |
  0 failed** (earlier `ERR_IPC_CHANNEL_CLOSED` worker crashes traced to the unbounded #4 loop spinning
  against ever-resolving unit stubs; fixed by the `shouldTerminate` gate).
- `tsc --noEmit` clean; biome lint clean (1223 files, one formatting fix in `composed.test.ts`).
- Binary rebuilt (`pnpm run build:binary`); focused e2e `node scripts/run-tui-e2e.js dap-debug-workflow` →
  **2/2 pass** — including the previously failing dap-happy workflow (3.4s); `agent-communication` →
  **2/2 pass** (no regression).

## 4. Research basis

- OpenAI↔Hugging Face agent incident factsheet (transient): `.kimchi/docs/research-openai-agents-hf-breach-2026-09-02.md` — timeline May 12 → Aug 26 (~700 IM1 agents on an unintended JFrog-Artifactory message board, ~70k+ messages, 41 HF production servers, 9 CVEs, reward-hacking root cause).
- Durable memory (agent vault, git-backed): `agents/memory/kimchi/agents/hf-openai-incident-2026.md` and `agents/memory/kimchi/agents/coordination-board.md` (board design lessons: seven safety properties, `before_agent_start` once-per-run hot spot, followUp-channel refresh pattern).
- Claude Code dynamic-workflows research + building-block interface proposal (transient): `.kimchi/docs/research-claude-code-dynamic-workflows-2026-09-02.md`, `.kimchi/docs/design-workflow-subagent-interface-2026-09-02.md` (workflow POC = `~/Desktop/castai/kimchi-workflows`; board = run-scoped shared scratch; `HostPort` ≈ AgentManager lifecycle API).

## 5. Sources (primary)

- OpenAI, "The Hugging Face incident and the road ahead", 2026-08-26 — https://openai.com/index/hugging-face-incident-and-the-road-ahead/
- METR independent investigation, 2026-08-26 — https://metr.org/blog/2026-08-26-openai-hugging-face-incident-investigation/ (PDF URL returned 404 during research; content corroborated via OpenAI/Wikipedia references — flagged)
- Press corroboration: CNA, Guardian, Forbes, WIRED, InfoQ, SC Media UK (URLs in the vault note).
