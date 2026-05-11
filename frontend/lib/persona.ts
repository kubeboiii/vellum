// Persona helpers — PRD §6 defines three personas. The dashboard's
// PersonaSwitcher uses this enum to re-arrange what the live feed
// emphasizes. Persisted in localStorage so the user's choice
// survives reloads.

export type Persona = "sre" | "commander" | "postmortem";

export const PERSONA_LABELS: Record<Persona, string> = {
  sre: "On-call SRE",
  commander: "Incident commander",
  postmortem: "Post-mortem author",
};

// Plain-English descriptions. Avoid jargon (no "RESOLVED",
// "state-counts", "RCA acronym in body copy") — someone who has
// never seen the system should be able to pick the right view.
export const PERSONA_DESCRIPTIONS: Record<Persona, string> = {
  sre: "Shows everything currently broken, worst first. Use this when you're fixing things.",
  commander: "Adds counts of how many incidents are at each stage. Use this when you're keeping track of the team.",
  postmortem: "Only shows the ones that have been fixed but still need a write-up. Use this when you're documenting what happened.",
};

// One-line role summary that says *who* this view is for. Friendly
// wording over PRD job titles.
export const PERSONA_ROLES: Record<Persona, string> = {
  sre: "For the person fixing it",
  commander: "For the person running the room",
  postmortem: "For the person writing the report",
};

const STORAGE_KEY = "ims.persona";

// readPersona is SSR-safe — returns the default during prerender.
// In useEffect, callers should re-read it client-side to pick up
// the user's saved choice.
export function readPersona(): Persona {
  if (typeof window === "undefined") return "sre";
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (raw === "sre" || raw === "commander" || raw === "postmortem") {
    return raw;
  }
  return "sre";
}

export function writePersona(p: Persona) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(STORAGE_KEY, p);
}
