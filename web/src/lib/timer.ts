// src/lib/timer.ts
// Wall-clock-anchored countdown for the exam page. The remaining time is computed from
// Date.now() deltas against an absolute deadline, so background-tab throttling cannot make
// the timer undercount: when the tab refocuses, remaining() jumps to the correct value
// immediately instead of lagging behind (review findings #5, #10, Task 20).

export interface Countdown {
  /** Absolute deadline (ms epoch). */
  deadline: number;
  /** Remaining seconds, floored to >= 0. */
  remaining(): number;
  /** True once remaining() hits 0. */
  expired(): boolean;
}

/**
 * Build a countdown anchored to an absolute epoch deadline. `remaining()` recomputes from
 * Date.now() on every call, so it is always correct regardless of how long the tab was
 * backgrounded. Injecting a clock makes the pure logic unit-testable.
 */
export function makeCountdown(deadlineEpoch: number, now: () => number = Date.now): Countdown {
  return {
    deadline: deadlineEpoch,
    remaining: () => Math.max(0, Math.floor((deadlineEpoch - now()) / 1000)),
    expired: function () {
      return this.remaining() <= 0;
    }
  };
}

/**
 * Given a server-reported remaining-seconds and an optional client/server clock skew (ms,
 * signed = serverNow - clientNow), compute the absolute deadline so the countdown matches
 * the server's view of time.
 */
export function deadlineFromServerRemaining(serverRemainingSec: number, skewMs = 0, now: () => number = Date.now): number {
  return now() + skewMs + serverRemainingSec * 1000;
}

/** Format seconds as H:MM:SS (or M:SS when under an hour) for display. */
export function formatHMS(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const pad = (n: number) => String(n).padStart(2, '0');
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${m}:${pad(sec)}`;
}
