// useP0Beep — fires a short Web Audio beep when a new P0 incident
// appears in the polled `items` list, IF the user has unmuted.
//
// Why no MP3 asset: keeps the bundle small and avoids autoplay
// gotchas; the AudioContext is created lazily on the first beep,
// after a user gesture (their click on the mute toggle counts).

"use client";

import { useEffect, useRef } from "react";

import type { WorkItem } from "@/lib/types";

export function useP0Beep(items: WorkItem[], muted: boolean) {
  const knownP0Ids = useRef<Set<string> | null>(null);
  const ctxRef = useRef<AudioContext | null>(null);

  useEffect(() => {
    const currentP0 = new Set(
      items.filter((i) => i.severity === "P0").map((i) => i.id),
    );

    // First poll: snapshot, don't beep. Otherwise the page would
    // wail on every fresh load.
    if (knownP0Ids.current === null) {
      knownP0Ids.current = currentP0;
      return;
    }

    const novel = Array.from(currentP0).filter(
      (id) => !knownP0Ids.current!.has(id),
    );
    knownP0Ids.current = currentP0;

    if (muted || novel.length === 0) return;
    if (typeof window === "undefined") return;

    // Lazy-create the AudioContext on first beep. Some browsers
    // block ctx creation until a user gesture; the user already
    // clicked "unmute" before we get here.
    try {
      if (!ctxRef.current) {
        const Ctor =
          window.AudioContext ||
          (window as unknown as { webkitAudioContext: typeof AudioContext })
            .webkitAudioContext;
        if (!Ctor) return;
        ctxRef.current = new Ctor();
      }
      const ctx = ctxRef.current;
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = "square";
      osc.frequency.value = 880;
      // Quick attack, 120ms decay — short enough not to annoy.
      gain.gain.setValueAtTime(0.0001, ctx.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.08, ctx.currentTime + 0.01);
      gain.gain.exponentialRampToValueAtTime(
        0.0001,
        ctx.currentTime + 0.12,
      );
      osc.connect(gain).connect(ctx.destination);
      osc.start();
      osc.stop(ctx.currentTime + 0.13);
    } catch {
      // Audio unsupported / blocked — silently ignore.
    }
  }, [items, muted]);

  // Cleanup on unmount.
  useEffect(() => {
    return () => {
      ctxRef.current?.close().catch(() => {});
    };
  }, []);
}
